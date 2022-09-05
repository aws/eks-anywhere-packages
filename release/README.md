## EKS Anywhere Curated Packages Release Runbook

The steps to create a new release of EKS-A Package Controller

1. Tag the repository for example `git tag v0.2.2`
1. Push the tag `git push upstream :v0.2.2`
1. Update the build tooling:
    * Update the GIT_TAG https://github.com/aws/eks-anywhere-build-tooling/blob/main/projects/aws/eks-anywhere-packages/GIT_TAG
    * Run `make release` and update the CHECKSUMS file
    * Update the release in the README
    * Create a pull request and get it merged
1. Update the BUNDLE_NUMBER in https://github.com/aws/eks-anywhere/tree/release-<RELEASE-NUMBER>/release/triggers/bundle-release
    * Development
    * Production

