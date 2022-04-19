REPO_ROOT=$(shell git rev-parse --show-toplevel)
AWS_ACCOUNT_ID?=$(shell aws sts get-caller-identity --query Account --output text)
AWS_REGION?=us-west-2
SOURCE_CONTAINER_REGISTRY?=$(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com
RELEASE_CONTAINER_REGISTRY?=$(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com
BUILD_REPO_BRANCH_NAME?=main
CLI_REPO_BRANCH_NAME?=main
GOLANG_VERSION?="1.17"
GO ?= $(shell source $(REPO_ROOT)/scripts/common.sh && build::common::get_go_path $(GOLANG_VERSION))/go
ECR_PUBLIC_URI?=$(shell AWS_REGION=us-east-1 && aws ecr-public describe-registries --query 'registries[*].registryUri' --output text)
KMS_KEY=signingPackagesKey

BASE_DIRECTORY=$(shell git rev-parse --show-toplevel)
GIT_HASH=$(shell git -C $(BASE_DIRECTORY) rev-parse HEAD)


ifeq ($(CODEBUILD_CI),)
DRY_RUN=true
SOURCE_BUCKET=projectbuildpipeline-857-pipelineoutputartifactsb-10ajmk30khe3f
else
DRY_RUN=false
endif

IMG ?= controller:latest

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

all: build

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

clean: ## Clean output directory, and the built binary
	rm -rf output/
	rm -rf bin/generatebundlefile

##@ Build

build: ## Build release binary.
	mkdir -p $(REPO_ROOT)/generatebundlefile/bin
	$(GO) mod tidy -compat=1.17
	$(GO) build -o $(REPO_ROOT)/generatebundlefile/bin/generatebundlefile *.go

build-linux:
	[ -d bin ] || mkdir bin
	env GOOS=linux GOARCH=amd64 $(MAKE) build

run:
	$(GO) run .

# CI for Bundle Creation and Signing

priv-login:
	aws ecr get-login-password --region us-west-2 | HELM_EXPERIMENTAL_OCI=1 helm registry login --username AWS --password-stdin $(AWS_ACCOUNT_ID).dkr.ecr.us-west-2.amazonaws.com

public-login:
	aws ecr-public get-login-password --region us-east-1 | HELM_EXPERIMENTAL_OCI=1 helm registry login --username AWS --password-stdin public.ecr.aws

dev-promote: build priv-login public-login
	$(REPO_ROOT)/generatebundlefile/scripts/promote.sh

dev-release:
	$(REPO_ROOT)/generatebundlefile/scripts/generate_bundle.sh $(ECR_PUBLIC_URI) $(KMS_KEY)

test:
	$(GO) test

## ORAS Make commands
ORAS_VERSION := "0.12.0"
ifeq ($(OS),Windows_NT)
        # no op
else
	UNAME_S := $(shell uname -s)
	UNAME_M := $(shell uname -m)
	ifeq ($(UNAME_S),Linux)
		ifeq ($(UNAME_M),x86_64)
			ORAS_OSARCH := "linux_amd64"
		endif
	endif
	ifeq ($(UNAME_S),Darwin)
		ifeq ($(UNAME_M),x86_64)
			ORAS_OSARCH := "darwin_amd64"
		endif
	endif
endif

.PHONY: oras-install
oras-install:
	curl -L -s https://github.com/oras-project/oras/releases/download/v$(ORAS_VERSION)/oras_$(ORAS_VERSION)_$(ORAS_OSARCH).tar.gz \
		| tar zx -C $(REPO_ROOT)/bin oras