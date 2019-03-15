package list_issues

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/github"
	cli "github.com/jawher/mow.cli"
	"golang.org/x/oauth2"
	"gopkg.in/cheggaaa/pb.v1"
)

var (
	Version = "dev"
	Build   = "not-specified"
)

var (
	regexIssue = regexp.MustCompile(`(([a-z0-9]+)\/([a-z0-9]+))?#([0-9]+)`)
	issues     = make(map[string]*github.Issue)
)

func Issue(owner, name, id string) *github.Issue {
	key := fmt.Sprintf("%s/%s#%s", owner, name, id)
	issue, ok := issues[key]
	if !ok {
		issue = &github.Issue{}
		issues[key] = issue
		number, err := strconv.ParseInt(id, 10, 32)
		if err != nil {
			panic(err)
		}
		issue.Number = Int(int(number))
		r := Repository(owner, name)
		issue.Repository = r
		issues[key] = issue
	}
	return issue
}

var repositories = make(map[string]*github.Repository)

func Repository(owner, name string) *github.Repository {
	key := fmt.Sprintf("%s/%s", owner, name)
	r, ok := repositories[key]
	if !ok {
		r = &github.Repository{
			Owner: &github.User{
				Name: Str(owner),
			},
			Name: Str(name),
		}
		repositories[key] = r
	}
	return r
}

var (
	regExpRepInfoFromURL = regexp.MustCompile("^(git@github.com:([a-z0-9]+)/([a-z0-9]+).git|https://github.com/([a-z0-9]+)/([a-z0-9]+))$")
)

func getRepositoryInfoFromURL(currentDir string) (string, string) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = currentDir
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
	url := strings.TrimSpace(out.String())
	matches := regExpRepInfoFromURL.FindStringSubmatch(url)
	if len(matches) == 0 {
		panic(fmt.Errorf("%s: is not a valid URL", url))
	}
	if matches[2] != "" {
		return matches[2], matches[3]
	}
	return matches[4], matches[5]
}

func sortIssues(lst *[]*github.Issue) {
	sort.SliceStable(*lst, func(i, j int) bool {
		return (*lst)[i].ClosedAt.Before(*(*lst)[j].ClosedAt)
	})
}

type IssueCategory struct {
	Label  string
	Text   string
	Issues []*github.Issue
}

var (
	categorizedIssues        = make(map[string]*IssueCategory)
	categorizedIssuesOrdered = make([]*IssueCategory, 0)
)

func NewCategorizedIssue(label string) *IssueCategory {
	s := strings.Split(label, ":")
	c := &IssueCategory{
		Issues: make([]*github.Issue, 0),
	}
	if len(s) > 1 {
		c.Label = s[0]
		c.Text = s[1]
	} else {
		c.Label = s[0]
		c.Text = s[0]
	}
	categorizedIssues[c.Label] = c
	categorizedIssuesOrdered = append(categorizedIssuesOrdered, c)
	return c
}

var (
	argsApp                  = cli.App("list-issues", "List issues between two commits/branches/tags")
	argCompare               = argsApp.StringArg("COMPARE", "", "Ref used in the git log. Ex: master..issue-32, issue-323..HEAD^^.")
	optVerbose               = argsApp.BoolOpt("verbose v", false, "Verbose mode.")
	optToken                 = argsApp.StringOpt("token t", "", "Token that will provide permission for acessing the issues. **Required** for private repositories (you can generate https://github.com/settings/tokens).")
	optLabels                = argsApp.StringsOpt("labels l", []string{"enhancement:Enhancements", "bug:Bugs", "!:Other"}, "The sessions based on labels. If you set bug:Bugs as a label, it will format set the session header as `Bugs`. ! matches any other issue.")
	optOnlyClosed            = argsApp.BoolOpt("only-closed c", true, "Include only closed issues.")
	optIncludeExternalIssues = argsApp.BoolOpt("external-issues e", true, "Include issues from outside of this repository")
	optDisplaySummary        = argsApp.BoolOpt("summary s", true, "Display summary")
)

func init() {
	argsApp.Spec = "[COMPARE] [-v][-t][-l][-c][-e][-s]"
}

func Verbose(args ...interface{}) {
	if *optVerbose {
		fmt.Fprintln(os.Stderr, args...)
	}
}

func Verbosef(format string, args ...interface{}) {
	if *optVerbose {
		fmt.Fprintf(os.Stderr, format, args...)
	}
}

func main() {
	fmt.Fprintln(os.Stderr, "Version: %s", Version)
	fmt.Fprintln(os.Stderr, "Build: %s", Build)
	currentDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	defaultRepositoryOwner, defaultRepositoryName := getRepositoryInfoFromURL(currentDir)

	argsApp.Action = func() {
		var defaultCategory *IssueCategory

		for _, label := range *optLabels {
			c := NewCategorizedIssue(label)
			if c.Label == "!" {
				defaultCategory = c
			}
		}

		gitArgs := []string{"log", `--pretty=format:%H%n%B%n---`}
		if *argCompare != "" {
			gitArgs = append(gitArgs, *argCompare)
		}
		cmd := exec.Command("git", gitArgs...)
		cmd.Dir = currentDir
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			panic(err)
		}

		ctx := context.Background()

		var tc *http.Client
		// If a token is informed...
		if *optToken != "" {
			ts := oauth2.StaticTokenSource(
				&oauth2.Token{AccessToken: *optToken},
			)
			tc = oauth2.NewClient(ctx, ts)
		}
		client := github.NewClient(tc)

		reader := bufio.NewReader(&out)
		commitsFound := 0
		for {
			lineRaw, _, err := reader.ReadLine()
			if err == io.EOF {
				break
			} else if err != nil {
				panic(err)
			}
			commitSha1 := string(lineRaw)
			Verbosef("Commit %s found\n", commitSha1)
			body := bytes.NewBuffer(nil)
			for {
				lineRaw, _, err = reader.ReadLine()
				if err == io.EOF {
					break
				} else if err != nil {
					panic(err)
				}
				line := string(lineRaw)
				if line == "---" {
					break
				}
				Verbose("    ", line)
				body.Write(lineRaw)
			}
			commitsFound++
			Verbose("")

			matches := regexIssue.FindAllStringSubmatch(body.String(), 50)
			for _, s := range matches {
				if s[2] == "" {
					s[2] = defaultRepositoryOwner
				} else if (s[2] != defaultRepositoryOwner || s[3] != defaultRepositoryName) && !*optIncludeExternalIssues { // If !empty, it is an external issue.
					Verbose("Ignoring external issue:", s[0])
					continue
				}
				if s[3] == "" {
					s[3] = defaultRepositoryName
				}
				Issue(s[2], s[3], s[4])
			}
		}

		fmt.Fprintf(os.Stderr, "Commits found: %d\n", commitsFound)
		fmt.Fprintln(os.Stderr, "Fetching issues information...")

		time.Sleep(time.Millisecond * 100)

		bar := pb.StartNew(len(issues))
		bar.Output = os.Stderr
		bar.SetWidth(80)
		for _, issue := range issues {
			bar.Increment()
			ctx := context.Background()

			opt := &github.RepositoryListByOrgOptions{
				ListOptions: github.ListOptions{PerPage: 10},
			}
			issueDetailed, resp, err := client.Issues.Get(ctx, *issue.Repository.Owner.Name, *issue.Repository.Name, int(*issue.Number))
			if err != nil {
				panic(err)
			}
			opt.Page = resp.NextPage

			// If not closed and execution flag "only-closed" enabled.
			if issueDetailed.ClosedAt == nil && *optOnlyClosed {
				continue
			}

			issueMatchedCategory := false
			for _, l := range issueDetailed.Labels {
				// If the label matches any categorized issue
				if c, ok := categorizedIssues[*l.Name]; ok {
					c.Issues = append(c.Issues, issueDetailed)
					issueMatchedCategory = true
					break
				}
			}
			if !issueMatchedCategory && defaultCategory != nil {
				defaultCategory.Issues = append(defaultCategory.Issues, issueDetailed)
			}
		}

		fmt.Fprintln(os.Stderr)
		time.Sleep(time.Millisecond * 100)

		for _, category := range categorizedIssuesOrdered {
			sortIssues(&category.Issues)
			fmt.Printf("### %s\n", category.Text)
			for _, issue := range category.Issues {
				fmt.Printf("* #%d: %s;\n", *issue.Number, *issue.Title)
			}
			fmt.Println()
		}

		if *optDisplaySummary {
			fmt.Println("")
			totalSummarized := 0
			for _, category := range categorizedIssuesOrdered {
				fmt.Printf("%s: %d\n", category.Text, len(category.Issues))
				totalSummarized += len(category.Issues)
			}
			fmt.Printf("Total: %d\n", totalSummarized)
		}
	}

	err = argsApp.Run(os.Args)
	if err != nil {
		panic(err)
	}
}

func Str(s string) *string {
	return &s
}

func Int(i int) *int {
	return &i
}
