# Generate Bundle Files

## Overview

This binary reads an input file describing the curated packages to be included in the bundle then generates a bundle custom resource file.

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

### Input File Supported Formats

Currently the following formats can be used as input files with each of the following command flags.

--key alias/signingPackagesKey

#### Private Registry

```yaml
name: "v1-22-1001"
kubernetesVersion: "1.22"
packages:
  - org: aws-containers
    projects:
      - name: hello-eks-anywhere
        repository: hello-eks-anywhere
        registry: 646717423341.dkr.ecr.us-west-2.amazonaws.com
        versions:
            - name: latest
```

#### Public Registry

```yaml
name: "v1-22-1001"
kubernetesVersion: "1.22"
packages:
  - org: aws-containers
    projects:
      - name: hello-eks-anywhere
        repository: hello-eks-anywhere
        registry: public.ecr.aws/eks-anywhere
        versions:
            - name: latest
```

#### Latest Version

The "latest" tag will tell the program to use a timestamp lookup, and find the most recently pushed helm chart for information, even if that helm chart doesn't have the latest tag.

```yaml
name: "v1-22-1001"
kubernetesVersion: "1.22"
packages:
  - org: aws-containers
    projects:
      - name: hello-eks-anywhere
        repository: hello-eks-anywhere
        registry: public.ecr.aws/eks-anywhere
        versions:
            - name: latest
```

#### Named Version

```yaml
name: "v1-22-1001"
kubernetesVersion: "1.22"
packages:
  - org: aws-containers
    projects:
      - name: hello-eks-anywhere
        repository: hello-eks-anywhere
        registry: public.ecr.aws/eks-anywhere
        versions:
            - name: 0.1.1-083e68edbbc62ca0228a5669e89e4d3da99ff73b
```

#### Multiple Versions

```yaml
name: "v1-22-1001"
kubernetesVersion: "1.22"
packages:
  - org: aws-containers
    projects:
      - name: hello-eks-anywhere
        repository: hello-eks-anywhere
        registry: public.ecr.aws/eks-anywhere
        versions:
            - name: 0.1.1-92904119e6e1bae35bf88663d0875259d42346f8
            - name: 0.1.1-083e68edbbc62ca0228a5669e89e4d3da99ff73b
```

#### Mixed Format (Latest + Named)

```yaml
name: "v1-22-1001"
kubernetesVersion: "1.22"
packages:
  - org: aws-containers
    projects:
      - name: hello-eks-anywhere
        repository: hello-eks-anywhere
        registry: public.ecr.aws/eks-anywhere
        versions:
            - name: 0.1.1-92904119e6e1bae35bf88663d0875259d42346f8
            - name: latest
```
