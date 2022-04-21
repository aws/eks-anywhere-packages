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

# More Information about the Docker Cred Helper https://github.com/docker/docker-credential-helpers
# ECR credential helper automatically uses the same credentials as the AWS CLI and the AWS SDKs 
# to first retrieve an ECR authentication token for each account for secure access to repositories,
# then lets the Docker daemon use this token when familiar Docker commands such as docker push or docker pull
if [ ! -d "/root/.docker" ]; then
    mkdir -p /root/.docker
fi

if [ ! -d "/root/.config/containers" ]; then
    mkdir -p /root/.config/containers
fi

mv generatebundlefile/hack/policy.json /root/.config/containers/policy.json
mv generatebundlefile/hack/docker-ecr-config.json /root/.docker/config.json
git config --global credential.helper '!aws codecommit credential-helper $@'
git config --global credential.UseHttpPath true