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

cat << EOF > configfile
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

${BASE_DIRECTORY}/generatebundlefile/bin/generatebundlefile  \
    --input ${BASE_DIRECTORY}/generatebundlefile/data/staging_artifact_move.yaml \
    --private-profile ${PROFILE}

# Release Helm Chart, and bundle to Staging account
cat << EOF > stagingconfigfile
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

# Move Helm charts within the bundle to another account
${BASE_DIRECTORY}/generatebundlefile/bin/generatebundlefile  \
    --input ${BASE_DIRECTORY}/generatebundlefile/data/staging_artifact_move.yaml \
    --public-profile ${PROFILE}

if [ ! -x "${ORAS_BIN}" ]; then
    make oras-install
fi

function generate () {
    local version=$1
    local kms_key=signingPackagesKey

    cd "${BASE_DIRECTORY}/generatebundlefile"
    ./bin/generatebundlefile --input "./data/bundles_staging/${version}.yaml" \
                 --key alias/${kms_key} \
                 --output "output-${version}"
}

function regionCheck () {
    local version=$1
    cd "${BASE_DIRECTORY}/generatebundlefile"
    ./bin/generatebundlefile --bundle "output/${version}"/bundle.yaml \
                --region-check true || true
}

function push () {
    local version=${1?:no version specified}
    cd "${BASE_DIRECTORY}/generatebundlefile/output-${version}"
    awsAuth "$REPO" | "$ORAS_BIN" login "$REPO" --username AWS --password-stdin
    "$ORAS_BIN" pull "${REPO}:v${version}-latest" -o ${version}
    removeBundleMetadata ${version}/bundle.yaml
    removeBundleMetadata bundle.yaml
    if (git diff --no-index --quiet -- ${version}/bundle.yaml.stripped bundle.yaml.stripped) then
        echo "bundle contents are identical skipping bundle push for ${version}"
    else
        "$ORAS_BIN" push "${REPO}:v${version}-${CODEBUILD_BUILD_NUMBER}" bundle.yaml
        "$ORAS_BIN" push "${REPO}:v${version}-latest" bundle.yaml
    fi
}

for version in 1-20 1-21 1-22 1-23 1-24; do
    generate ${version}
done

export AWS_PROFILE=staging
aws ecr-public get-login-password --region us-east-1 | HELM_EXPERIMENTAL_OCI=1 helm registry login --username AWS --password-stdin public.ecr.aws

for version in 1-20 1-21 1-22 1-23 1-24; do
    regionCheck ${version}
    push ${version}
done
