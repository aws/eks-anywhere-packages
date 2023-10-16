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

cat << EOF > configfile
[default]
region=us-west-2
account=$BASE_AWS_ACCOUNT_ID
output=json

[profile packages]
role_arn=$PACKAGES_ARTIFACT_DEPLOYMENT_ROLE
region=us-west-2
credential_source=EcsContainer
EOF

# Release Package Images to Packages Artifact account
BASE_DIRECTORY=$(git rev-parse --show-toplevel)
export AWS_CONFIG_FILE=${BASE_DIRECTORY}/generatebundlefile/configfile
export PROFILE=packages
AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
. "${BASE_DIRECTORY}/generatebundlefile/hack/common.sh"
ORAS_BIN=${BASE_DIRECTORY}/bin/oras

make build
chmod +x ${BASE_DIRECTORY}/generatebundlefile/bin/generatebundlefile

aws ecr get-login-password --region us-west-2 | HELM_EXPERIMENTAL_OCI=1 helm registry login --username AWS --password-stdin ${AWS_ACCOUNT_ID}.dkr.ecr.us-west-2.amazonaws.com
aws ecr-public get-login-password --region us-east-1 | HELM_EXPERIMENTAL_OCI=1 helm registry login --username AWS --password-stdin public.ecr.aws

# Move Helm Images within the bundle to Private ECR
${BASE_DIRECTORY}/generatebundlefile/bin/generatebundlefile  \
    --input ${BASE_DIRECTORY}/generatebundlefile/data/staging_artifact_move.yaml \
    --private-profile ${PROFILE}

# Release Helm Chart, and bundle to Staging account
cat << EOF > stagingconfigfile
[default]
region=us-west-2
account=$BASE_AWS_ACCOUNT_ID
output=json

[profile staging]
role_arn=$ARTIFACT_DEPLOYMENT_ROLE
region=us-east-1
credential_source=EcsContainer
EOF

export AWS_CONFIG_FILE=${BASE_DIRECTORY}/generatebundlefile/stagingconfigfile
export PROFILE=staging
. "${BASE_DIRECTORY}/generatebundlefile/hack/common.sh"
ECR_PUBLIC=$(aws ecr-public --region us-east-1 describe-registries --profile ${PROFILE} --query 'registries[*].registryUri' --output text)
REPO=${ECR_PUBLIC}/eks-anywhere-packages-bundles

aws ecr-public get-login-password --region us-east-1 | HELM_EXPERIMENTAL_OCI=1 helm registry login --username AWS --password-stdin public.ecr.aws

# Move Helm charts within the bundle to Public ECR
${BASE_DIRECTORY}/generatebundlefile/bin/generatebundlefile  \
    --input ${BASE_DIRECTORY}/generatebundlefile/data/staging_artifact_move.yaml \
    --public-profile ${PROFILE}

if [ ! -x "${ORAS_BIN}" ]; then
    make oras-install
fi

# Generate Bundles from Public ECR
export AWS_PROFILE=staging
export AWS_CONFIG_FILE=${BASE_DIRECTORY}/generatebundlefile/stagingconfigfile
for version in 1-23 1-24 1-25 1-26 1-27 1-28; do
    generate ${version} "staging"
done

# Push Bundles to Public ECR
aws ecr-public get-login-password --region us-east-1 | HELM_EXPERIMENTAL_OCI=1 helm registry login --username AWS --password-stdin public.ecr.aws
for version in 1-23 1-24 1-25 1-26 1-27 1-28; do
    push ${version}
done

# Check images from Bundle in Private ECR
export AWS_CONFIG_FILE=${BASE_DIRECTORY}/generatebundlefile/configfile
export AWS_PROFILE=packages
for version in 1-23 1-24 1-25 1-26 1-27 1-28; do
    regionCheck ${version}
done
