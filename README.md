# List issues

This is a helper tool for listing all issues mentioned in commits between two
references (branches, versions).

## Building

This command will download all dependencies needed and compile a binary to the
`bin` directory. Add this folder to your `$PATH`. 

```
make dep-ensure install
```

:warning: Tested on Go 1.11.4 under Deepin 15.9.2. Other Go versions + OS might
compile with no problems. 

## Usage

```
list-commits [COMPARE] [-v][-t][-l][-c][-e][-s][-h]
```

* `COMPARE`                 : Ref..ref passed to the git log to generate the list of commits. Ex: `master..issue-323`.
* `-v`, `--verbose`         : Default: `false`. Verbose mode.
* `-t`, `--token`           : Token that will provide permission for acessing the issues. **Required** for private repositories (you can generate https://github.com/settings/tokens).
* `-l`, `--labels`          : Default: `enhancement:Enhancements`, `bug:Bugs`, `!:Other`. The sessions based on labels. If you set `bug:Bugs` as a label, it will format set the session header as "Bugs". `!` matches any other issue. 
* `-c`, `--only-closed`     : Default: `true`. Include only closed issues. 
* `-e`, `--external-issues` : Default: `true`. Include issues from outside of this repository.
* `-s`, `--summary`         : Default: `true`. Display summary.

## Output example

```
list-issues master..release-0.7.0
```

```
### Enhancements
* #222: Implements the first version of the AppBot models;
* #162: Define AppBot and its capabilities;
...
* #435: Add Edge Event Error;

### Bugs
* #317: Fix schema recovery when creating AppBotVersion;
* #323: Moves the account identifier from header to the body for AppBots requests;
* #315: Move the account identifier from header to the body for AppBots requests;
...
* #424: Enable handover assignment rules to loop through arrays;
* #430: Add loop protection in Graph Deserializer;

### Other
* #224: Implement services for AppBot;
* #246: Merge master into issue-220;
...
* #421: Driver update payload data + handover fixed;
* #422: Add support for Images in Argo Driver;

Enhancements: 67
Bugs: 18
Other: 21
Total: 106
```

### License

MIT