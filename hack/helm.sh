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
./bin/kustomize build config/crd > charts/eks-anywhere-packages-crds/templates/crd.yaml

cd charts
helm lint eks-anywhere-packages
OUTDIR=_output
rm -rf $OUTDIR
mkdir $OUTDIR
cp -r eks-anywhere-packages $OUTDIR
EKS_ANYWHERE_PACKAGES_TAG=$(cat image-tags/eks-anywhere-packages)
sed \
        -e "s,{{eks-anywhere-packages}},${EKS_ANYWHERE_PACKAGES_TAG}," \
        eks-anywhere-packages/values.yaml >${OUTDIR}/eks-anywhere-packages/values.yaml
cd $OUTDIR
RESULT=$(helm package eks-anywhere-packages| sed -e 's/Successfully packaged chart and saved it to: //g')
echo "helm install eks-anywhere-packages ${RESULT}"
