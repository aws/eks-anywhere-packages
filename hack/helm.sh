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
set -o errexit
set -o nounset
set -o pipefail

ROOT_DIR="$(git rev-parse --show-toplevel)"

cd ${ROOT_DIR}
./bin/kustomize build config/crd > charts/eks-anywhere-packages/crds/crd.yaml

cp api/testdata/packagebundlecontroller.yaml api/testdata/packagecontroller.yaml api/testdata/bundle_one.yaml charts/eks-anywhere-packages/templates
cd charts
helm lint eks-anywhere-packages
helm package eks-anywhere-packages
helm-docs
