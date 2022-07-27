SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

REPO_ROOT=$(shell git rev-parse --show-toplevel)
GOLANG_VERSION?="1.17"
GO ?= $(shell source $(REPO_ROOT)/scripts/common.sh && build::common::get_go_path $(GOLANG_VERSION))/go

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: build

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

clean: ## Clean output directory, and the built binary
	rm -rf output/
	rm -rf bin/ecr-refresh

##@ Build

build: ## Build Binary
	mkdir -p $(REPO_ROOT)/ecrtokenrefresher/bin
	$(GO) mod tidy -compat=1.17
	$(GO) build -o $(REPO_ROOT)/ecrtokenrefresher/bin/ecr-refresh *.go

build-linux:
	[ -d bin ] || mkdir bin
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(MAKE) build

run:
	$(GO) run .
