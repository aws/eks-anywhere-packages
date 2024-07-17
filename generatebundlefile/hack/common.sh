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

# orasLogin echoes an AWS ECR password
function orasLogin () {
    local repo=${1?:no repo specified}
    local awsCmd="ecr"
    local region="--region=us-west-2"

    if [[ $repo =~ "public.ecr.aws" ]]; then
        awsCmd="ecr-public"
        region="--region=us-east-1"
    fi

    profile=${PROFILE:-}
    profile_arg=""
    if [ -n "$profile" ]; then
        profile_arg="--profile=$profile"
    fi

    aws "$awsCmd" "$region" $profile_arg get-login-password | "$ORAS_BIN" login "$repo" --username AWS --password-stdin
}

function generate () {
    local version=$1
    local stage=$2
    local kms_key="arn:aws:kms:us-west-2:857151390494:alias/signingPackagesKey"

    file_name=${version}.yaml

    cd "${BASE_DIRECTORY}/generatebundlefile"
    ./bin/generatebundlefile --input "./data/bundles_${stage}/$file_name" \
                 --key ${kms_key} \
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
    local stage=${2?:no version specified}
    cd "${BASE_DIRECTORY}/generatebundlefile/output-${version}"
    orasLogin "$REPO"

    latest_tag="v${version}-latest"
    versioned_tag=$(yq ".metadata.name" bundle.yaml)
    case $stage in
    dev)
        latest_tag="${latest_tag}"
        versioned_tag="${versioned_tag}"
        ;;
    staging)
        latest_tag="${latest_tag}-staging"
        versioned_tag="${versioned_tag}-staging"
        ;;
    prod)
        latest_tag="${latest_tag}-prod"
        versioned_tag="${versioned_tag}-prod"
        ;;
    *)
        echo "Invalid stage: $stage"
        exit 1
        ;;
    esac

    "$ORAS_BIN" push "${REPO}:${versioned_tag}" bundle.yaml
    "$ORAS_BIN" push "${REPO}:${latest_tag}" bundle.yaml

}
