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

# skopeoLogin authenticates the Skopeo command with an ECR or ECR Public registry
function skopeoLogin () {
    local repo=${1?:no repo specified}
    local awsCmd="ecr"
    local region="--region=us-west-2"

    registry=$(echo $repo | cut -d'/' -f1)
    if [[ $repo =~ "public.ecr.aws" ]]; then
        awsCmd="ecr-public"
        region="--region=us-east-1"
    fi

    regional_build_mode=${REGIONAL_BUILD_MODE:-}
    if [[ "$regional_build_mode" == "true" ]]; then
        profile=default
        export AWS_PROFILE=$profile
    else
        profile=${PROFILE:-}
    fi

    aws "$awsCmd" "$region" --profile=$profile get-login-password | skopeo login "$registry" --username AWS --password-stdin
}

function generate () {
    local version=$1
    local stage=$2
    local kms_key="arn:aws:kms:us-west-2:857151390494:alias/signingPackagesKey"

    file_name=${version}.yaml
    regional_build_mode=${REGIONAL_BUILD_MODE:-}
    if [ "$regional_build_mode" == "true" ]; then
        file_name=${version}-regional.yaml
    fi

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
    cd "${BASE_DIRECTORY}/generatebundlefile/output-${version}"
    bundle_sha256sum=$(sha256sum bundle.yaml | awk '{print $1}')
    skopeoLogin "$REPO"
    
    current_latest_bundle_sha256sum=""
    if skopeo copy docker://"${REPO}:v${version}-latest" dir://$PWD/skopeo-old-${version}; then
        current_latest_bundle_sha256sum="$(cat $PWD/skopeo-old-${version}/manifest.json | jq -r '.layers[0].digest')"
    fi

    if [[ "sha256:$bundle_sha256sum" = "$current_latest_bundle_sha256sum" ]]; then
        echo "Bundle contents are identical, skipping bundle push for ${version}"
    else
        echo "Pushing bundle to $REPO"
        mkdir -p $PWD/skopeo-new-${version}
        cp bundle.yaml skopeo-new-${version}/$bundle_sha256sum
        empty_json_sha256sum=$(echo -n '{}' | sha256sum | awk '{print $1}')
        echo -n '{}' > skopeo-new-${version}/$empty_json_sha256sum
        echo "Directory Transport Version: 1.1" > skopeo-new-${version}/version
        bundle_size=$(cat bundle.yaml | wc -c | awk '{print $1}')
        jq -nc \
            --arg config_digest "$empty_json_sha256sum" \
            --arg bundle_digest "$bundle_sha256sum" \
            --arg bundle_size $bundle_size \
            '{"schemaVersion":2,"config":{"mediaType":"application/vnd.unknown.config.v1+json","digest":"sha256:\($config_digest)","size":2},"layers":[{"mediaType":"application/vnd.oci.image.layer.v1.tar","digest":"sha256:\($bundle_digest)","size":$bundle_size|tonumber,"annotations":{"org.opencontainers.image.title":"bundle.yaml"}}]}' | tr -d '\n' > skopeo-new-${version}/manifest.json
        skopeo copy dir://$PWD/skopeo-new-${version} docker://"${REPO}:v${version}-${CODEBUILD_BUILD_NUMBER}" -f oci --all
        skopeo copy dir://$PWD/skopeo-new-${version} docker://"${REPO}:v${version}-latest" -f oci --all
    fi
}
