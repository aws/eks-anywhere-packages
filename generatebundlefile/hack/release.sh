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
make build

cat << EOF > configfile
[profile prod]
role_arn=$PROD_ARTIFACT_DEPLOYMENT_ROLE
region=us-east-1
credential_source=EcsContainer
EOF

export AWS_CONFIG_FILE=configfile

KMS_KEY=signingPackagesKey
PROFILE=prod
IMAGE_REGISTRY=$(aws ecr-public --region us-east-1 describe-registries --profile ${PROFILE} --query 'registries[*].registryUri' --output text)
AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

aws ecr get-login-password --region us-west-2 | HELM_EXPERIMENTAL_OCI=1 helm registry login --username AWS --password-stdin ${AWS_ACCOUNT_ID}.dkr.ecr.us-west-2.amazonaws.com
aws ecr-public get-login-password --region us-east-1 | HELM_EXPERIMENTAL_OCI=1 helm registry login --username AWS --password-stdin public.ecr.aws

# Release the bundle to another account
${BASE_DIRECTORY}/generatebundlefile/bin/generatebundlefile  \
    --input ${BASE_DIRECTORY}/generatebundlefile/data/input_121.yaml \
    --release-profile ${PROFILE}

# Create the prod bundle for 1.21
${BASE_DIRECTORY}/generatebundlefile/bin/generatebundlefile  \
    --input ${BASE_DIRECTORY}/generatebundlefile/data/input_121_prod.yaml \
    --key alias/${KMS_KEY}

make oras-install

. "${BASE_DIRECTORY}/common.sh"
cd ${BASE_DIRECTORY}/generatebundlefile/output
awsAuth ${BASE_DIRECTORY}/bin/oras push --username AWS --password-stdin \
    "${IMAGE_REGISTRY}/eks-anywhere-packages-bundles:v1-21-${CODEBUILD_BUILD_NUMBER}" \
    bundle.yaml
awsAuth ${BASE_DIRECTORY}/bin/oras push --username AWS --password-stdin \
    "${IMAGE_REGISTRY}/eks-anywhere-packages-bundles:v1-21-latest" \
    bundle.yaml

# 1.22 Bundle Build
cd ${BASE_DIRECTORY}/generatebundlefile

${BASE_DIRECTORY}/generatebundlefile/bin/generatebundlefile  \
    --input ${BASE_DIRECTORY}/generatebundlefile/data/input_122_prod.yaml \
    --key alias/${KMS_KEY}

cd ${BASE_DIRECTORY}/generatebundlefile/output
awsAuth ${BASE_DIRECTORY}/bin/oras push --username AWS --password-stdin \
    "${IMAGE_REGISTRY}/eks-anywhere-packages-bundles:v1-22-${CODEBUILD_BUILD_NUMBER}" \
    bundle.yaml
awsAuth ${BASE_DIRECTORY}/bin/oras push --username AWS --password-stdin \
    "${IMAGE_REGISTRY}/eks-anywhere-packages-bundles:v1-22-latest" \
    bundle.yaml
