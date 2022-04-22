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

curl -OL https://github.com/sigstore/cosign/releases/download/v1.7.2/cosign-linux-amd64

chmod +x cosign-linux-amd64
mkdir -p ${BASE_DIRECTORY}/bin
mv cosign-linux-amd64 ${BASE_DIRECTORY}/bin/cosign
${BASE_DIRECTORY}/bin/cosign version

# Create the bundle
${BASE_DIRECTORY}/generatebundlefile/bin/generatebundlefile  \
    --input ${BASE_DIRECTORY}/generatebundlefile/data/input_120.yaml

# Sign the Bundle
export AWS_REGION="us-west-2"
SIGNATURE=$(${BASE_DIRECTORY}/bin/cosign sign-blob --key awskms:///alias/${KMS_KEY} output/bundle-1.20.yaml)

# Add signature annotation
${BASE_DIRECTORY}/generatebundlefile/bin/generatebundlefile  \
    --input ${BASE_DIRECTORY}/generatebundlefile/output/bundle-1.20.yaml \
    --signature ${SIGNATURE}

make oras-install

ECR_PASSWORD=$(aws ecr-public get-login-password --region us-east-1)
cd output/
${BASE_DIRECTORY}/bin/oras push -u AWS -p "${ECR_PASSWORD}" "${IMAGE_REGISTRY}/eks-anywhere-packages-bundles:v1" bundle-1.20.yaml