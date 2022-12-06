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

# Common functions used by various build scripts. To use these
# functions, source this file, e.g.:
#    . <path-to-this-file>/common.sh

# awsAuth echoes an AWS ECR password
function awsAuth () {
    local repo=${1?:no repo specified}
    local awsCmd="ecr"
    local region="--region=us-west-2"

    if [[ $repo =~ "public.ecr.aws" ]]; then
        awsCmd="ecr-public"
        region="--region=us-east-1"
    fi

    aws "$awsCmd" "$region" "--profile=${PROFILE:-}" get-login-password
}

function generate () {
    local version=$1
    local stage=$2
    local kms_key=signingPackagesKey

    cd "${BASE_DIRECTORY}/generatebundlefile"
    ./bin/generatebundlefile --input "./data/bundles_${stage}/${version}.yaml" \
                 --key alias/${kms_key} \
                 --output "output-${version}"
}

function regionCheck () {
    local version=$1
    cd "${BASE_DIRECTORY}/generatebundlefile"
    ./bin/generatebundlefile --bundle "output-${version}/bundle.yaml" \
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

function removeBundleMetadata () {
    local bundle=${1?:no bundle specified}
    yq 'del(.metadata.name)' ${bundle} > ${bundle}.strippedname
    yq 'del(.metadata.annotations)' ${bundle}.strippedname > ${bundle}.stripped
}
