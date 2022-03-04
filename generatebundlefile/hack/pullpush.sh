#!/usr/bin/env bash
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


export JSONPATH="imageDetails[?(@.imageTags[?(@ == '"${IMAGE_TAG}"')])].imageDigest"
aws ecr-public describe-images --region us-east-1 --repository-name cert-manager-controller  --output text  --query "${JSONPATH}"

aws ecr-public create-repository --repository-name cert-manager-controller
aws ecr-public create-repository --repository-name eks-anywhere-test

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

helm pull oci://public.ecr.aws/l0g8r8j6/eks-anywhere-test --version v0.1.1-4280284ae5696ef42fd2a890d083b88f75d4978a-helm
helm push eks-anywhere-test-v0.1.1-4280284ae5696ef42fd2a890d083b88f75d4978a-helm.tgz "oci://$local_ecr_public"
