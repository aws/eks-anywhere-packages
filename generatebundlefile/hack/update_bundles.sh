#!/bin/bash

set -e
set -o pipefail

BASE_DIRECTORY=$(git rev-parse --show-toplevel)
# Function to display usage information
function usage() {
  echo "Usage: $0 -e <environment>"
  echo "  -e    Specify the environment (staging or prod)"
  exit 1
}

# Function to check if required tools are installed
function check_requirements() {
  local required_tools=("aws" "jq" "yq")
  for tool in "${required_tools[@]}"; do
    if ! command -v "$tool" &>/dev/null; then
      echo "Error: $tool is not installed or not in PATH" >&2
      exit 1
    fi
  done
}

# Function to get the latest version from ECR
function get_latest_version() {
  local repo=$1
  local k8s_version=$2

  local json_output=$(aws ecr describe-images \
    --repository-name "$repo" \
    --output json)

  # For metrics-server and cluster autoscaler k8s version is embedded in image tag
  # Cluster Autosclaer has version format 1.XX and metrics-server has format 1-XX
  # For cluster autoscaler filter tag based on k8s version (1.28,1.29,etc)
  if [ "$repo" = "cluster-autoscaler/charts/cluster-autoscaler" ]; then
    kube_version="1.$k8s_version"
  # For metrics-server filter tag based on k8s version (1-28,1-29,etc)
  elif [ "$repo" = "metrics-server/charts/metrics-server" ]; then
    kube_version="1-$k8s_version"
  fi

  if [ -n "${kube_version:-}" ]; then
    output=$(echo $json_output | jq -r --arg kv "$kube_version" '
    .imageDetails
    | map(select(.imageTags | length > 0))
    | map(select(.imageTags | map(test($kv)) | any))
    | sort_by(.imagePushedAt)
    | last
    | .imageTags')
  else
    output=$(echo $json_output | jq -r '
    .imageDetails
    | sort_by(.imagePushedAt)
    | last(.[]).imageTags')
  fi

  # Convert tags to array
  local versions=()
  for value in $(echo "$output" | jq -r '.[]'); do
    versions+=("$value")
  done

  # Get latest version and remove tags with helm
  local latest_version=$(printf "%s\n" "${versions[@]}" |
    grep -v "helm" |
    sort -rV |
    head -n 1)

  echo $latest_version
}

# Function to update input bundle file for a given Kubernetes version
function update_bundle() {
  k8s_version=$1
  environment=$2
  k8s_yaml_file="${BASE_DIRECTORY}/generatebundlefile/data/bundles_$environment/1-$k8s_version.yaml"
  packages=$(yq e '.packages' "$k8s_yaml_file")
  package_count=$(echo "$packages" | yq e 'length' -)

  echo "Updating input file for kubernetes version: 1-$k8s_version"
  # Iterate over all packages in yaml file
  for package_index in $(seq 0 $(($package_count - 1))); do
    package=$(echo "$packages" | yq e ".[$package_index]" -)
    org=$(echo "$package" | yq e '.org' -)
    projects=$(echo "$package" | yq e '.projects' -)
    project_count=$(echo "$projects" | yq e 'length' -)

    # Iterate over each project in org
    for project_index in $(seq 0 $((project_count - 1))); do
      project=$(echo "$projects" | yq e ".[$project_index]" -)

      # Extract registry and repository from each project
      registry=$(echo "$project" | yq e '.registry' -)
      repository=$(echo "$project" | yq e '.repository' -)

      # Get the latest version for the repository
      latest_tag=$(get_latest_version $repository $k8s_version)

      if [ "$latest_tag" == "None" ]; then
        echo "No tags found for repository: $repository in registry $registry. Skipping..."
        continue
      fi

      yq e -i ".packages[$package_index].projects[$project_index].versions[0].name |= \"$latest_tag\"" "$k8s_yaml_file"
      echo "Updated $org/$repository to latest_tag $latest_tag"
    done
  done
}

# Function to get supported k8s versions for EKS-A
function get_supported_versions() {
  local versions
  versions=$(curl -s https://raw.githubusercontent.com/aws/eks-anywhere-build-tooling/main/release/SUPPORTED_RELEASE_BRANCHES |
    sed 's/1-//' |
    sort -V)

  local supported_versions=$(printf "%s\n" "${versions[@]}")
  echo $supported_versions
}

function main() {
  local environment=""
  # Parse command line options
  while getopts "e:" opt; do
    case ${opt} in
    e)
      environment=$OPTARG
      ;;
    \?) # Invalid Option
      usage
      exit
      ;;
    esac
  done

  # Check if environment is provided and valid
  if [[ -z "$environment" ]]; then
    echo "Error: Environment not specified" >&2
    usage
  fi

  if [[ "$environment" != "staging" && "$environment" != "prod" ]]; then
    echo "Error: Invalid environment. Use 'staging' or 'prod'" >&2
    usage
  fi

  # Check if required tools(aws,jq,yq) are installed
  check_requirements

  # Generate bundle files for supported k8s versions (we fetch supported k8s versions from https://github.com/aws/eks-anywhere-build-tooling/blob/main/release/SUPPORTED_RELEASE_BRANCHES)
  for version in $(get_supported_versions); do
    update_bundle "$version" "$environment"
  done

  echo "Bundle update complete."

}

main "$@"
