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

function set_aws_config() {
    release_environment="$1"
    if [ "$release_environment" = "production" ]; then
        if [ "$PROD_ARTIFACT_DEPLOYMENT_ROLE" = "" ]; then
            echo "Empty PROD_ARTIFACT_DEPLOYMENT_ROLE"
            exit 1
        fi
        cat <<EOF >>awscliconfig
[profile artifacts-production]
role_arn=$PROD_ARTIFACT_DEPLOYMENT_ROLE
region=us-east-1
credential_source=EcsContainer
EOF
    fi

    export AWS_CONFIG_FILE=$(pwd)/awscliconfig
}

# Setup AWS profile for publishing message to SNS
set_aws_config "production"

# Get supported RELEASE_BRANCH
FILE_URL="https://raw.githubusercontent.com/aws/eks-anywhere-build-tooling/main/release/SUPPORTED_RELEASE_BRANCHES"
RELEASE_BRANCH=$(curl -s "$FILE_URL" | grep -v '^[[:space:]]*$' | head -n 1)

# Perform oras pull
oras pull public.ecr.aws/eks-anywhere/eks-anywhere-packages-bundles:v${RELEASE_BRANCH}-latest

# Extract date from bundle.yaml
if [ -f "bundle.yaml" ]; then
    # Extract the date portion from metadata.name in bundle.yaml
    LATEST_PACKAGE_BUNDLE_DATE=$(yq eval '.metadata.name' bundle.yaml | cut -d'-' -f3,4,5)
    echo "Bundle date: ${BUNDLE_DATE}"
else
    echo "Error: bundle.yaml not found"
    exit 1
fi

NOTIFICATION_SUBJECT="New release of EKS Anywhere Curated Packages Bundle ($LATEST_PACKAGE_BUNDLE_DATE)"
NOTIFICATION_BODY="A new Amazon EKS Anywhere curated packages bundle has been released on $LATEST_PACKAGE_BUNDLE_DATE. You can find the latest bundle artifacts in the https://gallery.ecr.aws/eks-anywhere/eks-anywhere-packages-bundles. The release includes updated container images and Helm charts for the supported EKS Anywhere add-ons and packages. The EKS Anywhere package controller periodically checks upstream for the latest package bundle and applies it to your management cluster, for airgapped environment you would have to get the package bundle manually from outside of the airgapped environment and apply it to your management cluster. Follow this guide to manage your package bundle: https://anywhere.eks.amazonaws.com/docs/packages/packagebundles/"

echo "
Sending SNS message with the following details:
    - Topic ARN: $EKSA_CURATED_PACKAGES_RELEASE_SNS_TOPIC_ARN
    - Subject: $NOTIFICATION_SUBJECT
    - Body: $NOTIFICATION_BODY"

# Publish release message to SNS
SNS_MESSAGE_ID=$(
    aws sns publish \
        --topic-arn "$EKSA_CURATED_PACKAGES_RELEASE_SNS_TOPIC_ARN" \
        --subject "$NOTIFICATION_SUBJECT" \
        --message "$NOTIFICATION_BODY" \
        --query "MessageId" \
        --output text
)

if [ "$SNS_MESSAGE_ID" ]; then
  echo -e "\nRelease notification published with SNS MessageId $SNS_MESSAGE_ID"
else
  echo "Received unexpected response while publishing to release SNS topic. An error may have occurred, and the \
notification may not have not have been published"
  exit 1
fi