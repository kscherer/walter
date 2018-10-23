NAME := walter
VERSION := $(shell grep 'Version string' version.go | sed -E 's/.*"(.+)"$$/\1/')
REVISION := $(shell git rev-parse --short HEAD)
LDFLAGS := -X 'main.GitCommit=$(REVISION)'
PKGS := . ./lib/*

setup:
	go get golang.org/x/lint/golint
	go get golang.org/x/tools/cmd/goimports
	go get github.com/mitchellh/gox

deps: setup
	dep ensure

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

