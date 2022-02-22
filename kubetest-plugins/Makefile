# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

GIT_TAG?=$(shell git describe --tags --always --dirty 2>/dev/null)

all: vet lint build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: fmt
fmt: ## Run gofmt against code.
	gofmt -w .
	gci -w -local github.com/aws/eks-anywhere-packages/kubetest-plugins $(shell find . -name '*go')

.PHONY: lint
lint: ../bin/golangci-lint ## Run golangci-lint
	../bin/golangci-lint run

../bin/golangci-lint: ## Download golangci-lint
	make -C .. bin/golangci-lint

vet: ## Run go vet against code.
	go mod tidy
	go vet ./...

clean: ## Clean up resources created by make targets
	rm -rf ./bin/*
	rm -rf cover.out
	rm -rf testbin

##@ Build

build: vet
	go build -o bin/kubetest2-eksa -ldflags="-X=github.com/aws/eks-anywhere-packages/kubetest-plugins/cmd/kubetest2-eksa/deployer.GitTag=$(GIT_TAG)" ./cmd/kubetest2-eksa
	go build -o bin/kubetest2-tester-eksa -ldflags="-X=github.com/aws/eks-anywhere-packages/kubetest-plugins/cmd/kubetest2-tester-eksa/main.GitTag=$(GIT_TAG)" ./cmd/kubetest2-tester-eksa
