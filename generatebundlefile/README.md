# Generate Bundle Files

## Overview

This binary reads an input file describing the curated packages to be included in the bundle then generates a bundle custom resource file. In addition, it has the ability to promote images and Helm charts between container registries.

There are three types of helm version tags we can input:

1. Exact name match. A tag named `v1.0.1.1345` will return the SHA checksum if there is exact name match.
2. A substring version with `-latest` at the end of the name. In this case we'll search for the most recent Helm chart starting with `v0.1.1` and return that SHA checksum.
3. A tag of `latest` will return the last Helm chart pushed to this repository, and its tag and SHA checksum.

This will output two CRD objects for the projects. One named after item in the project list.

## Build

```sh
make build
```

## Run

### Bundle Generation

To generate a signed bundle you need an AWS KMS key that is:
- Asymmetric
- ECC_NIST_P256
- Key Policy enabled for CLI user to have `kms:Sign` configured. 

```sh
generatebundlefile --input data/sample_input.yaml --key alias/signingPackagesKey
```

This will output all the corresponding CRD's into `output/bundle.yaml` 

#### Sample Bundle Generation

```sh
generatebundlefile --generate-sample
```

This will output a sample bundle file to ./output.

### Package Promotion

To promote a package from a private ECR to public you need the repository name. This repository must contain a Helm chart built by the process in the eks-anywhere-build-tooling git repository.

```sh
generatebundlefile --promote hello-eks-anywhere
```
