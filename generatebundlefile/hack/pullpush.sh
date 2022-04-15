#!/usr/bin/env bash
# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#!/usr/bin/env bash


###################

# This file is NOT meant to be run as a script, or in CI. It's for useful commands when developing/testing generatebundle.

####################

set -e

export local_ecr_public="public.ecr.aws/f5b7k4z5"

function repoExists () {
    aws ecr-public describe-repositories \
        | jq -r ".repositories[].repositoryName" \
        | grep -q "^$1\$"
}

REPO_1=hello-eks-anywhere
REPO_2=eks-anywhere-packages
if ! repoExists ${REPO_1}; then
      aws ecr-public  create-repository --repository-name ${REPO_1}
fi

if ! repoExists ${REPO_2}; then
      aws ecr-public create-repository --repository-name ${REPO_2}
fi

#############
### ECR Logins
#############

export HELM_EXPERIMENTAL_OCI=1
aws ecr-public get-login-password --region us-east-1 | HELM_EXPERIMENTAL_OCI=1 helm registry login --username AWS --password-stdin public.ecr.aws
aws ecr get-login-password --region us-west-2 | HELM_EXPERIMENTAL_OCI=1 helm registry login --username AWS --password-stdin 646717423341.dkr.ecr.us-west-2.amazonaws.com

################################################
# Populate Test cases for generateBundle
# TODO replace version this with an upstream version from the dev Account once it's published.
#################################################

helm pull oci://646717423341.dkr.ecr.us-west-2.amazonaws.com/aws-containers/hello-eks-anywhere --version 0.1.0+c4e25cb42e9bb88d2b8c2abfbde9f10ade39b214
helm push hello-eks-anywhere-0.1.0+c4e25cb42e9bb88d2b8c2abfbde9f10ade39b214.tgz "oci://$local_ecr_public"

########################
# Oras examples
#########################

aws ecr-public get-login-password --region us-east-1 | oras login \
    --username AWS \
    --password-stdin public.ecr.aws

# Manually push a local bundle
cd output/
oras push public.ecr.aws/f5b7k4z5/eks-anywhere-packages-bundles:v1.0.0-bundle bundle-1.20.yaml

# Pull latest bundle
oras pull public.ecr.aws/f5b7k4z5/eks-anywhere-packages-bundles:v1

########################
# Hack to Simulate Codebuild jobs locally, Run Builder-base test of promo/signing flags
########################

go mod vendor
docker run -d -v ${HOME}/.aws/credentials:/root/.aws/credentials:ro -v ${HOME}/go/src/github.com/modelrocket/eks-anywhere-packages:/go/src/github.com/aws/eks-distro/eks-anywhere-packages public.ecr.aws/eks-distro-build-tooling/builder-base:latest  tail -f /dev/null
docker ps
docker exec -it <CONTAINER_ID> /bin/bash
./generateBundle/scripts/setup.sh

# Change the `make build` command to use vendor

build: ## Build release binary.
	mkdir -p $(REPO_ROOT)/generatebundlefile/bin
	$(GO) build -mod vendor -o $(REPO_ROOT)/generatebundlefile/bin/generatebundlefile *.go

make build-linux
export HELM_REPO=aws-containers/hello-eks-anywhere
make dev-promote