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

BASE_AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
BASE_DIRECTORY=$(git rev-parse --show-toplevel)

. "${BASE_DIRECTORY}/generatebundlefile/hack/common.sh"
ORAS_BIN=${BASE_DIRECTORY}/bin/oras

make build
chmod +x ${BASE_DIRECTORY}/generatebundlefile/bin/generatebundlefile

. "${BASE_DIRECTORY}/generatebundlefile/hack/common.sh"

REGISTRY=${BASE_AWS_ACCOUNT_ID}.dkr.ecr.us-west-2.amazonaws.com
REPO=${REGISTRY}/eks-anywhere-packages-bundles

if [ ! -x "${ORAS_BIN}" ]; then
    make oras-install
fi

# Generate bundles from beta account private ECR registry and
# push them to prod account private ECR registry (same as beta account in this case)
export AWS_PROFILE=prod
for version in 1-26 1-27 1-28 1-29 1-30; do
    generate ${version} "prod"
    push ${version} "prod"
done
