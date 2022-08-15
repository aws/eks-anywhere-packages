## Amazon EKS Anywhere Curated Packages

[![Release](https://img.shields.io/github/v/release/aws/eks-anywhere-packages.svg?logo=github&color=green)](https://github.com/aws/eks-anywhere-packages/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/aws/eks-anywhere-packages)](https://goreportcard.com/report/github.com/aws/eks-anywhere-packages)
[![Contributors](https://img.shields.io/github/contributors/aws/eks-anywhere-packages?color=purple)](CONTRIBUTING.md)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

**Build status:** ![Build status](https://codebuild.us-west-2.amazonaws.com/badges?uuid=eyJlbmNyeXB0ZWREYXRhIjoiRmp0cVVpck53WjVxYUVibGxFdSsxM05sby9zenRkN1YwRTVLTjhBUUFORXpGQkVkR2Y3aThhdDhEN3pHZzRpRHl0K2xRcFd0U2VIcWpUaW9kb1hOV3FFPSIsIml2UGFyYW1ldGVyU3BlYyI6InNKTm5MNWZPNVA3T0tOV0EiLCJtYXRlcmlhbFNldFNlcmlhbCI6MX0%3D&branch=main)

---
The Amazon EKS Anywhere Curated Packages are only available to customers with the Amazon EKS Anywhere Enterprise Subscription. To request a free trial, talk to your Amazon representative or connect with one [here](https://aws.amazon.com/contact-us/sales-support-eks/).

---

EKS Anywhere Curated Packages is a management system for installation, configuration and maintenance of additional components for your Kubernetes cluster. Examples of these components may include Container Registry, Ingress, and LoadBalancer, etc.

Here are the steps for [getting started](docs/README.md) with EKS Anywhere Curated Packages.

## Development

EKS Anywhere Curated Packages is tested using
[Prow](https://github.com/kubernetes/test-infra/tree/master/prow), the Kubernetes CI system.
EKS operates an installation of Prow, which is visible at [https://prow.eks.amazonaws.com/](https://prow.eks.amazonaws.com/).
Please read our [CONTRIBUTING](CONTRIBUTING.md) guide before making a pull request.

The dependencies which make up EKS Anywhere Curated Packages are defined and built via the [build-tooling](https://github.com/aws/eks-anywhere-build-tooling) repo.

## Security

If you discover a potential security issue in this project, or think you may
have discovered a security issue, we ask that you notify AWS Security via our
[vulnerability reporting page](http://aws.amazon.com/security/vulnerability-reporting/).
Please do **not** create a public GitHub issue.

## License

This project is licensed under the [Apache-2.0 License](LICENSE).
