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

IMAGE_REGISTRY="${1?Specify first argument - image registry}"
KMS_KEY="${2?Specify second argument - kms key alias}"

BASE_DIRECTORY=$(git rev-parse --show-toplevel)
chmod +x ${BASE_DIRECTORY}/generatebundlefile/bin/generatebundlefile 

# Faster way to install Cosign compared to go install github.com/sigstore/cosign/cmd/cosign@v1.5.1
curl -s https://api.github.com/repos/sigstore/cosign/releases/latest \
| grep 'browser_download_url.*cosign-linux-amd64"' \
| cut -d '"' -f 4 \
| tr -d \" \
| xargs curl -OL

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