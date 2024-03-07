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
export CODEBUILD_BUILD_NUMBER=$(($CODEBUILD_BUILD_NUMBER+110))

. "${BASE_DIRECTORY}/generatebundlefile/hack/common.sh"
ORAS_BIN=${BASE_DIRECTORY}/bin/oras

make build
chmod +x ${BASE_DIRECTORY}/generatebundlefile/bin/generatebundlefile

regional_build_mode=${REGIONAL_BUILD_MODE:-}
if [[ "$regional_build_mode" != "true" ]]; then
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
    export AWS_CONFIG_FILE=${BASE_DIRECTORY}/generatebundlefile/configfile
    export PROFILE=packages
    AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

    aws ecr get-login-password --region us-west-2 | HELM_EXPERIMENTAL_OCI=1 helm registry login --username AWS --password-stdin ${AWS_ACCOUNT_ID}.dkr.ecr.us-west-2.amazonaws.com
    aws ecr-public get-login-password --region us-east-1 | HELM_EXPERIMENTAL_OCI=1 helm registry login --username AWS --password-stdin public.ecr.aws

    # Move Helm Images within the bundle to Private ECR
    ${BASE_DIRECTORY}/generatebundlefile/bin/generatebundlefile  \
        --input ${BASE_DIRECTORY}/generatebundlefile/data/prod_artifact_move.yaml \
        --private-profile ${PROFILE}
fi

# Release Helm Chart, and bundle to Production account
cat << EOF > prodconfigfile
[default]
region=us-west-2
account=$BASE_AWS_ACCOUNT_ID
output=json

[profile prod]
role_arn=$ARTIFACT_DEPLOYMENT_ROLE
region=us-east-1
credential_source=EcsContainer
EOF

export AWS_CONFIG_FILE=${BASE_DIRECTORY}/generatebundlefile/prodconfigfile
export PROFILE=prod
. "${BASE_DIRECTORY}/generatebundlefile/hack/common.sh"

aws ecr-public get-login-password --region us-east-1 | HELM_EXPERIMENTAL_OCI=1 helm registry login --username AWS --password-stdin public.ecr.aws

if [ "$regional_build_mode" == "true" ]; then
    file_name=prod_artifact_move-regional.yaml
    REGISTRY=${BASE_AWS_ACCOUNT_ID}.dkr.ecr.us-west-2.amazonaws.com
else
    file_name=prod_artifact_move.yaml
    REGISTRY=$(aws ecr-public --region us-east-1 describe-registries --profile ${PROFILE} --query 'registries[*].registryUri' --output text)
fi
REPO=${REGISTRY}/eks-anywhere-packages-bundles

# Move Helm charts within the bundle to Public ECR
${BASE_DIRECTORY}/generatebundlefile/bin/generatebundlefile  \
    --input ${BASE_DIRECTORY}/generatebundlefile/data/${file_name} \
    --public-profile ${PROFILE}

if [ ! -x "${ORAS_BIN}" ]; then
    make oras-install
fi

# Generate Bundles from Public ECR
export AWS_PROFILE=prod
export AWS_CONFIG_FILE=${BASE_DIRECTORY}/generatebundlefile/prodconfigfile
for version in 1-25 1-26 1-27 1-28 1-29; do
    generate ${version} "prod"
done

# Push Bundles to Public ECR
aws ecr-public get-login-password --region us-east-1 | HELM_EXPERIMENTAL_OCI=1 helm registry login --username AWS --password-stdin public.ecr.aws
for version in 1-25 1-26 1-27 1-28 1-29; do
    push ${version}
done
