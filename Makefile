# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# Image URL to use all building/pushing image targets
IMG ?= controller:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false"

GOLANG_VERSION?="1.17"
GO ?= $(shell source ./scripts/common.sh && build::common::get_go_path $(GOLANG_VERSION))/go

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell $(GO) env GOPATH)/bin
else
GOBIN=$(shell $(GO) env GOBIN)
endif

all: presubmit

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
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run gofmt against code.
	gofmt -w $(shell find . -name '*go' | grep -v mock)
	gci -w -local github.com/aws/eks-anywhere-packages $(shell find . -name '*go' | grep -v mock)

.PHONY: lint
lint: bin/golangci-lint ## Run golangci-lint
	bin/golangci-lint run

bin/golangci-lint: ## Download golangci-lint
bin/golangci-lint: GOLANGCI_LINT_VERSION?=$(shell cat .github/workflows/golangci-lint.yml | sed -n -e 's/^\s*version: //p')
bin/golangci-lint:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s $(GOLANGCI_LINT_VERSION)

.PHONY: vet
vet: ## Run go vet against code.
	$(GO) mod tidy
	$(GO) vet ./...

gosec: ## Run gosec against code.
	$(GO) install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec --exclude-dir=kubetest-plugins --exclude-dir generatebundlefile  ./...

SIGNED_ARTIFACTS = pkg/signature/testdata/packagebundle_valid.yaml.signed pkg/signature/testdata/pod_valid.yaml.signed api/testdata/bundle_one.yaml.signed api/testdata/bundle_two.yaml.signed
ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
# Test a specific package with something like ./api/... see go help packages for
# full syntax details.
GOTESTS ?= ./...
# Use "-short" to skip long tests, or "-verbose" for more verbose reporting. Run
# go help testflags to see all options.
GOTESTFLAGS ?= ""
test: manifests generate vet mocks ${SIGNED_ARTIFACTS} ## Run tests.
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.8.3/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); $(GO) test $(GOTESTFLAGS) `$(GO) list $(GOTESTS) | grep -v mocks` -coverprofile cover.out

clean: ## Clean up resources created by make targets
	rm -rf ./bin/*
	rm -rf cover.out
	rm -rf testbin
	rm -rf charts/_output
	$(MAKE) -C kubetest-plugins clean

##@ Build

build: generate vet ## Build package-manager binary.
	$(GO) build -o bin/package-manager main.go

run: manifests generate vet ## Run a controller from your host.
	$(GO) run ./main.go

docker-build: test ## Build docker image with the package-manager.
	docker build -t ${IMG} .

docker-push: ## Push docker image with the package-manager.
	docker push ${IMG}

helm-build: kustomize ## Build helm chart into tar file
	hack/helm.sh

##@ Deployment

install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	kubectl get namespace eksa-packages || kubectl create namespace eksa-packages
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl delete -f -
	kubectl get namespace eksa-packages && kubectl delete namespace eksa-packages

deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | kubectl delete -f -

helm-deploy:
	helm upgrade --install eksa-packages charts/eks-anywhere-packages/
	
helm-delete:
	helm delete eksa-packages

CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1)

KUBETEST2 = $(shell pwd)/bin/kubetest2
kubetest2: ## Download kubetest2 locally if necessary.
	$(call go-get-tool,$(KUBETEST2),sigs.k8s.io/kubetest2@latest)

KUSTOMIZE = $(shell pwd)/bin/kustomize
kustomize: ## Download kustomize locally if necessary.
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v3@v3.8.7)

MOCKGEN = $(shell pwd)/bin/mockgen
mockgen: ## Download mockgen locally if necessary.
	$(call go-get-tool,$(MOCKGEN),github.com/golang/mock/mockgen@v1.6.0)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
$(GO) mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin $(GO) get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

## Generate mocks
.PHONY: mocks
mocks: mockgen controllers/mocks/client.go controllers/mocks/manager.go pkg/driver/mocks/packagedriver.go pkg/bundle/mocks/bundle_client.go pkg/packages/mocks/manager.go

pkg/bundle/mocks/bundle_client.go: pkg/bundle/bundle_client.go
	PATH=$(shell $(GO) env GOROOT)/bin:$$PATH \
		$(MOCKGEN) -source pkg/bundle/bundle_client.go -destination=pkg/bundle/mocks/bundle_client.go -package=mocks BundleClient

pkg/packages/mocks/manager.go: pkg/packages/manager.go
	PATH=$(shell $(GO) env GOROOT)/bin:$$PATH \
		$(MOCKGEN) -source pkg/packages/manager.go -destination=pkg/packages/mocks/manager.go -package=mocks Manager

pkg/driver/mocks/packagedriver.go: pkg/driver/packagedriver.go
	PATH=$(shell $(GO) env GOROOT)/bin:$$PATH \
		$(MOCKGEN) -source pkg/driver/packagedriver.go -destination=pkg/driver/mocks/packagedriver.go -package=mocks PackageDriver

controllers/mocks/client.go: go.mod
	PATH=$(shell $(GO) env GOROOT)/bin:$$PATH \
		$(MOCKGEN) -destination=controllers/mocks/client.go -package=mocks "sigs.k8s.io/controller-runtime/pkg/client" Client,StatusWriter

controllers/mocks/manager.go: go.mod
	PATH=$(shell $(GO) env GOROOT)/bin:$$PATH \
		$(MOCKGEN) -destination=controllers/mocks/manager.go -package=mocks "sigs.k8s.io/controller-runtime/pkg/manager" Manager

E2E_EKSA_PROVIDER ?= docker

.PHONY: test-e2e
test-e2e: kubetest-plugins
	PATH=$(CURDIR)/bin:$(CURDIR)/kubetest-plugins/bin:$$PATH kubetest2 eksa --cluster-name "e2e-test" --up --down --test eksa --provider $(E2E_EKSA_PROVIDER) -- --source-path=$(CURDIR)

.PHONY: test-e2e-smoke
test-e2e-smoke:
	bash -c 'aws ecr-public get-login-password --region us-east-1 | HELM_EXPERIMENTAL_OCI=1 helm registry login --username AWS --password-stdin public.ecr.aws'
	kubectl create namespace eksa-packages
	kubectl apply -f api/testdata/packagecontroller.yaml
	kubectl apply -f api/testdata/packagebundlecontroller.yaml
	kubectl apply -f api/testdata/bundle_one.yaml
	kubectl apply -f api/testdata/test.yaml
	@echo -E
	@echo -E
	@echo -E EKS-A Curated Packages E2E Test Successful
	@echo -E
	@echo -E

.PHONY: kubetest-plugins
kubetest-plugins: kubetest2
	$(MAKE) -C kubetest-plugins build

.PHONY: presubmit
presubmit: vet generate manifests build test # lint is run via github action

%.yaml.signed: %.yaml
	pkg/signature/testdata/sign_file.sh $?
