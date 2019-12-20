NAME := walter
VERSION := $(shell grep 'Version string' version.go | sed -E 's/.*"(.+)"$$/\1/')
REVISION := $(shell git rev-parse --short HEAD)
LDFLAGS := -X 'main.GitCommit=$(REVISION)'
PKGS := . ./lib/*

VERBOSE := 0
NO_CACHE := 0
TEST_CASE := ''

ifeq ($(TEST_CASE), '')
TEST_CASE_FLAG :=
else
TEST_CASE_FLAG := -run $(TEST_CASE)
endif

ifeq ($(NO_CACHE), 0)
GOTEST_FLAG := -race -count=1 lib/pipeline/pipeline_integration_test.go lib/pipeline/pipeline.go
else
GOTEST_FLAG := -race github.com/walter-cd/walter/lib/pipeline
endif

ifeq ($(VERBOSE), 0)
GOTESTSUM_FLAG := --format short-verbose
else
GOTESTSUM_FLAG := --format standard-verbose
endif

setup:
	go get golang.org/x/lint/golint
	go get golang.org/x/tools/cmd/goimports
	go get github.com/mitchellh/gox
	go get gotest.tools/gotestsum

deps: setup
	dep ensure

runtest: deps
	gotestsum $(GOTESTSUM_FLAG) -- $(TEST_CASE_FLAG) $(GOTEST_FLAG)

test: deps lint
	go test $(PKGS)
	go test -race $(PKGS)

lint: setup
	for pkg in $(PKGS); do \
		golint -set_exit_status $$pkg || exit $$?; \
	done

fmt: setup
	goimports -w $(PKGS)

build: deps
	go build -ldflags "$(LDFLAGS)" -o bin/$(NAME)

clean:
	rm $(GOPATH)/bin/$(NAME)
	rm bin/$(NAME)

package: deps
	@sh -c "'$(CURDIR)/scripts/package.sh'"

ghr:
	ghr -u walter-cd $(VERSION) pkg/dist/$(VERSION)

