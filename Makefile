
PROJECT := list-issues

# Define the `GOPATH` directory.
GOPATH=$(CURDIR)
GOBIN=$(CURDIR)/bin
GOPATHCMD=GOPATH=$(GOPATH) GOBIN=$(GOBIN)
GOCMD=$(GOPATHCMD) go

PROJECT_PATH=$(GOPATH)/src/github.com/jamillosantos/$(PROJECT)

.DEFAULT_GOAL: install

VERSION := `git describe --exact-match --tags 2> /dev/null || echo "dev-not-versioned"`
BUILD:=`git rev-parse HEAD`
VERSIONDOCKER := `git describe --exact-match --tags 2> /dev/null || echo "dev"`

# LD flags for linking
LDFLAGS=-X=main.Version=$(VERSION) -X=main.Build=$(BUILD)

TEST_FLAGS ?=

# Dep execution command
DEP=cd $(PROJECT_PATH) && GOPATH=$(GOPATH) dep

.PHONY: dep-ensure dep-update dep-add dep-status install

dep-ensure:
	@cd ${PROJECT_PATH} $(DEP) ensure -v

dep-update:
	@cd ${PROJECT_PATH} $(DEP) ensure -v -update $(PACKAGE)

dep-add:
ifdef PACKAGE
	@$(DEP) ensure -v -add $(PACKAGE)
else
	@echo "Usage: PACKAGE=<package url> make dep-add"
	@echo "The environment variable \`PACKAGE\` is not defined."
endif

dep-status:
	@$(DEP) status

install:
	@$(GOCMD) install "-ldflags=$(LDFLAGS)" -v $(PROJECT_PATH)
