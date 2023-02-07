# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# Image URL to use all building/pushing image targets
IMG ?= controller:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd"

GOLANG_VERSION?="1.18"
GO ?= $(shell source ./scripts/common.sh && build::common::get_go_path $(GOLANG_VERSION))/go

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell $(GO) env GOPATH)/bin
else
GOBIN=$(shell $(GO) env GOBIN)
endif

PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
BIN_DIR := $(PROJECT_DIR)/bin
GOLANGCI_LINT_CONFIG ?= .github/workflows/golangci-lint.yml
GOLANGCI_LINT := $(BIN_DIR)/golangci-lint

all: generate manifests build helm/package test # lint is run via github action

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

manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./api/v1alpha1" output:crd:artifacts:config=config/crd/bases

generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

$(GOBIN)/gci:
	$(GO) install github.com/daixiang0/gci@v0.8.0

.PHONY: fmt
fmt: run-gofmt run-gci

LS_FILES_CMD = git ls-files --exclude-standard | grep '\.go$$' | grep -v '/mocks/\|zz_generated\.'

.PHONY: run-gofmt
run-gofmt: ## Run gofmt against code.
	$(LS_FILES_CMD) | xargs gofmt -s -w

.PHONY: run-gci
run-gci: $(GOBIN)/gci ## Run gci against code.
	$(LS_FILES_CMD) | xargs $(GOBIN)/gci write --skip-generated -s standard,default -s "prefix($(shell go list -m))"

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Run golangci-lint
	$(GOLANGCI_LINT) run

$(GOLANGCI_LINT): $(BIN_DIR) $(GOLANGCI_LINT_CONFIG)
	$(eval GOLANGCI_LINT_VERSION?=$(shell cat .github/workflows/golangci-lint.yml | yq e '.jobs.golangci.steps[] | select(.name == "golangci-lint") .with.version' -))
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(BIN_DIR) $(GOLANGCI_LINT_VERSION)

go.sum: go.mod
	$(GO) mod tidy

.PHONY: vet
vet: ## Run go vet against code.
	$(GO) vet ./...

gosec: ## Run gosec against code.
	$(GO) install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec --exclude-dir generatebundlefile --exclude-dir ecrtokenrefresher  ./...

SIGNED_ARTIFACTS = pkg/signature/testdata/packagebundle_minControllerVersion.yaml.signed pkg/signature/testdata/packagebundle_valid.yaml.signed pkg/signature/testdata/pod_valid.yaml.signed api/testdata/bundle_one.yaml.signed api/testdata/bundle_two.yaml.signed
ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
# Test a specific package with something like ./api/... see go help packages for
# full syntax details.
GOTESTS ?= ./...
# Use "-short" to skip long tests, or "-verbose" for more verbose reporting. Run
# go help testflags to see all options.
# -buildmod=pie is temporary fix for https://github.com/golang/go/issues/54482
GOTESTFLAGS ?= "-buildmode=pie"
test: manifests generate mocks ${SIGNED_ARTIFACTS} ## Run tests.
	$(GO) test -vet=all $(GOTESTFLAGS) `$(GO) list $(GOTESTS) | grep -v mocks | grep -v fake | grep -v testutil` -coverprofile cover.out

clean: ## Clean up resources created by make targets
	rm -rf $(BIN_DIR)
	rm -rf cover.out
	rm -rf testbin
	rm -rf charts/_output

##@ Build

build: go.sum generate ## Build package-manager binary.
	$(GO) build -o $(BIN_DIR)/package-manager main.go

run: manifests generate vet ## Run a controller from your host.
	$(GO) run ./main.go server --verbosity 9

migrate: build ## Run a controller from your host.
	$(GO) run ./main.go migrate --verbosity 9

docker-build: test ## Build docker image with the package-manager.
	docker build -t ${IMG} .

docker-push: ## Push docker image with the package-manager.
	docker push ${IMG}

helm/build: helm-build
helm-build: kustomize ## Build helm chart into tar file
	hack/helm.sh
	helm-docs

helm/package: helm-package
helm-package: kustomize ## Build helm chart into tar file
	hack/helm.sh

##@ Deployment

install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	kubectl get namespace eksa-packages || kubectl create namespace eksa-packages
	$(KUSTOMIZE) build config/crd | kubectl apply -f -
	kubectl create secret -n eksa-packages generic aws-secret --from-literal=REGION=$(EKSA_AWS_REGION) --from-literal=ID=$(EKSA_AWS_ACCESS_KEY_ID) --from-literal=SECRET=$(EKSA_AWS_SECRET_ACCESS_KEY)

uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	kubectl delete packages -n eksa-packages $(kubectl get packages -n eksa-packages --no-headers -o custom-columns=":metadata.name") && sleep 5 || true
	helm delete eks-anywhere-packages || true
	$(KUSTOMIZE) build config/crd | kubectl delete -f - || true
	kubectl delete namespace eksa-packages || true

deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | kubectl delete -f -

helm-deploy:
	helm upgrade --install eksa-packages charts/eks-anywhere-packages/

helm-delete:
	helm delete eksa-packages

CONTROLLER_GEN = $(BIN_DIR)/controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.11.1)

KUSTOMIZE = $(BIN_DIR)/kustomize
kustomize: ## Download kustomize locally if necessary.
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v4@v4.5.7)

MOCKGEN = $(BIN_DIR)/mockgen
mockgen: ## Download mockgen locally if necessary.
	$(call go-get-tool,$(MOCKGEN),github.com/golang/mock/mockgen@v1.6.0)

# go-get-tool will 'go install' any package $2 and install it to $1.
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
$(GO) mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin $(GO) install $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

## Generate mocks
mocks: mockgen controllers/mocks/client.go controllers/mocks/manager.go
	PATH=$(BIN_DIR):$(PATH) go generate ./...

controllers/mocks/client.go: go.mod
	PATH=$(shell $(GO) env GOROOT)/bin:$$PATH \
		$(MOCKGEN) -destination=controllers/mocks/client.go -package=mocks "sigs.k8s.io/controller-runtime/pkg/client" Client,StatusWriter
controllers/mocks/manager.go: go.mod
	PATH=$(shell $(GO) env GOROOT)/bin:$$PATH \
		$(MOCKGEN) -destination=controllers/mocks/manager.go -package=mocks "sigs.k8s.io/controller-runtime/pkg/manager" Manager


.PHONY: presubmit
presubmit: vet generate manifests build helm/package test # targets for presubmit
	git --no-pager diff --name-only --exit-code ':!Makefile'

%.yaml.signed: %.yaml
	pkg/signature/testdata/sign_file.sh $?
