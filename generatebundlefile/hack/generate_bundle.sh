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

# Generate bundle files for supported kubernetes versions.

set -euxo pipefail

export LANG=C.UTF-8

BASE_DIRECTORY=$(git rev-parse --show-toplevel)
. "${BASE_DIRECTORY}/generatebundlefile/hack/common.sh"

AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
REGISTRY=${AWS_ACCOUNT_ID}.dkr.ecr.us-west-2.amazonaws.com
REPO=${REGISTRY}/eks-anywhere-packages-bundles
ORAS_BIN=${BASE_DIRECTORY}/bin/oras

if [ ! -x "${ORAS_BIN}" ]; then
    make oras-install
fi

make build
chmod +x ${BASE_DIRECTORY}/generatebundlefile/bin

for version in 1-28 1-29 1-30 1-31 1-32 1-33; do
    generate ${version} "dev"
    push ${version} "dev"
done
