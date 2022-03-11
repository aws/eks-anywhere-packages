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

set -x
set -e
set -o pipefail

USR_BIN=/usr/bin

if [ ! -d "/root/.docker" ]; then
    mkdir -p /root/.docker
fi
mv generatebundlefile/scripts/docker-ecr-config.json /root/.docker/config.json
git config --global credential.helper '!aws codecommit credential-helper $@'
git config --global credential.UseHttpPath true

#go install github.com/sigstore/cosign/cmd/cosign@v1.5.1

# Faster way to install Cosign compared to go install
curl -s https://api.github.com/repos/sigstore/cosign/releases/latest \
| grep 'browser_download_url.*cosign-linux-amd64"' \
| cut -d '"' -f 4 \
| tr -d \" \
| wget -qi -

chmod +x cosign-linux-amd64
cp cosign-linux-amd64 $USR_BIN/cosign
cosign version