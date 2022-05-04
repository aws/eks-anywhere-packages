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


set -e
set -x
set -o pipefail

export LANG=C.UTF-8

BASE_DIRECTORY=$(git rev-parse --show-toplevel)
chmod +x ${BASE_DIRECTORY}/generatebundlefile/bin/generatebundlefile 

IMAGE_REGISTRY=$(AWS_REGION=us-east-1 && aws ecr-public describe-registries --query 'registries[*].registryUri' --output text)
KMS_KEY=signingPackagesKey

# Create the bundle
${BASE_DIRECTORY}/generatebundlefile/bin/generatebundlefile  \
    --input ${BASE_DIRECTORY}/generatebundlefile/data/input_121.yaml \
    --key alias/${KMS_KEY}

mkdir -p ${BASE_DIRECTORY}/bin
make oras-install

ECR_PASSWORD=$(aws ecr-public get-login-password --region us-east-1)
cd output/
${BASE_DIRECTORY}/bin/oras push -u AWS -p "${ECR_PASSWORD}" "${IMAGE_REGISTRY}/eks-anywhere-packages-bundles:v1-21-${CODEBUILD_BUILD_NUMBER}" bundle.yaml
${BASE_DIRECTORY}/bin/oras push -u AWS -p "${ECR_PASSWORD}" "${IMAGE_REGISTRY}/eks-anywhere-packages-bundles:v1-21-latest" bundle.yaml