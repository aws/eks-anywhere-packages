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

#!/usr/bin/env bash


###################

# This file is NOT meant to be run as a script, or in CI. It's for useful commands when developing/testing generatebundle.

####################

set -e

aws ecr-public get-login-password --region us-east-1 | HELM_EXPERIMENTAL_OCI=1 helm registry login --username AWS --password-stdin public.ecr.aws

export local_ecr_public="public.ecr.aws/f5b7k4z5"
export name="cert-manager-controller"
export helm_name="cert-manager"
export tag_latest="v1.1.0-eks-a-latest"
export tag_1="v1.1.0-eks-a-4"
export tag_2="v1.1.0-eks-a-3"
export tag_3="v1.1.0-eks-a-2"
export tag_4="v1.1.0-eks-a-1"

function repoExists () {
    aws ecr-public describe-repositories \
        | jq -r ".repositories[].repositoryName" \
        | grep -q "^$1\$"
}

REPO_1=eks-anywhere-test
REPO_2=eks-anywhere-packages
if ! repoExists ${REPO_1}; then
      aws ecr-public  create-repository --repository-name ${REPO_1}
fi

if ! repoExists ${REPO_2}; then
      aws ecr-public create-repository --repository-name ${REPO_2}
fi

docker pull public.ecr.aws/eks-anywhere/jetstack/$name:$tag_1
docker pull public.ecr.aws/eks-anywhere/jetstack/$name:$tag_2
docker pull public.ecr.aws/eks-anywhere/jetstack/$name:$tag_3
docker pull public.ecr.aws/eks-anywhere/jetstack/$name:$tag_4


docker tag "public.ecr.aws/eks-anywhere/jetstack/$name:$tag_1" "$local_ecr_public/$name:$tag_1"
docker tag "public.ecr.aws/eks-anywhere/jetstack/$name:$tag_2" "$local_ecr_public/$name:$tag_2"
docker tag "public.ecr.aws/eks-anywhere/jetstack/$name:$tag_3" "$local_ecr_public/$name:$tag_3"
docker tag "public.ecr.aws/eks-anywhere/jetstack/$name:$tag_4" "$local_ecr_public/$name:$tag_4"

#Latest
docker tag "public.ecr.aws/eks-anywhere/jetstack/$name:$tag_1" "$local_ecr_public/$name:$tag_latest"

#Double tag scenario to test GetLatestSha Func
docker tag "public.ecr.aws/eks-anywhere/jetstack/$name:$tag_3" "$local_ecr_public/$name:$tag_1"
docker tag "public.ecr.aws/eks-anywhere/jetstack/$name:$tag_4" "$local_ecr_public/$name:$tag_3"
docker tag "public.ecr.aws/eks-anywhere/jetstack/$name:$tag_3" "$local_ecr_public/$name:$tag_2"




docker push "$local_ecr_public/$name:$tag_1"
docker push "$local_ecr_public/$name:$tag_2"
docker push "$local_ecr_public/$name:$tag_3"
docker push "$local_ecr_public/$name:$tag_4"
docker push "$local_ecr_public/$name:$tag_latest"


aws ecr-public batch-delete-image \
      --repository-name $name \
      --image-ids imageTag=$tag_4 \
      --region us-east-1

aws ecr-public batch-delete-image \
      --repository-name $name \
      --image-ids imageTag=$tag_2 \
      --region us-east-1

aws ecr-public batch-delete-image \
      --repository-name $name \
      --image-ids imageTag=$tag_latest \
      --region us-east-1


### Helm charts
export HELM_EXPERIMENTAL_OCI=1
aws ecr-public get-login-password --region us-east-1 | HELM_EXPERIMENTAL_OCI=1 helm registry login --username AWS --password-stdin public.ecr.aws
aws ecr get-login-password --region us-west-2 | HELM_EXPERIMENTAL_OCI=1 helm registry login --username AWS --password-stdin 646717423341.dkr.ecr.us-west-2.amazonaws.com

helm pull oci://646717423341.dkr.ecr.us-west-2.amazonaws.com/jetstack/cert-manager --version v1.0
helm push cert-manager-v1.0.tgz "oci://$local_ecr_public"
# sha256:950385098ceafc5fb510b1d203fa18165047598a09292a8f040b7812a882c256

helm pull oci://646717423341.dkr.ecr.us-west-2.amazonaws.com/jetstack/cert-manager --version v1.1
helm push cert-manager-v1.1.tgz "oci://$local_ecr_public"
# sha256:ce0e42ab4f362252fd7706d4abe017a2c52743c4b3c6e56a9554c912ffddebcd

helm pull oci://646717423341.dkr.ecr.us-west-2.amazonaws.com/jetstack/cert-manager --version v1.5.3-4ae27c6a1df646736d9e276358d0a6b2daf99f55
helm push cert-manager-v1.5.3-4ae27c6a1df646736d9e276358d0a6b2daf99f55.tgz "oci://$local_ecr_public"
# sha256:a507b9e9e739f6a2363b8739b1ad8f801b67768578867b9fae7d303f9e8918e8

# Populate test cases
public.ecr.aws/j0a1m4z9/eks-anywhere-packages --version 0.1.2-5010d89023bc2cdc520395b48d354c80ce2ad831-helm

helm pull oci://public.ecr.aws/l0g8r8j6/eks-anywhere-test --version 0.1.1-4280284ae5696ef42fd2a890d083b88f75d4978a-helm
mv eks-anywhere-test-0.1.1-4280284ae5696ef42fd2a890d083b88f75d4978a-helm.tgz eks-anywhere-test-v1.0.1-helm.tgz
helm push eks-anywhere-test-v1.0.1-helm.tgz "oci://$local_ecr_public"


helm push eks-anywhere-test-1.0.1.tgz "oci://$local_ecr_public"


# Oras
aws ecr-public get-login-password --region us-east-1 | oras login \
    --username AWS \
    --password-stdin public.ecr.aws

cd output/
oras push "public.ecr.aws/f5b7k4z5/eks-anywhere-test:v1.0.0-bundle" bundle-1.20.yaml