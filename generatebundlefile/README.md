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


### Public Promotion to another Account

```sh
generatebundlefile --private-profile "profile-name" --input data/sample_input.yaml
```

This command will move **Only** the helm chart from the listed input files to the target **public** ECR in another account.

### Private Promotion to another Account

```sh
generatebundlefile --private-profile "profile-name" --input data/sample_input.yaml
```

This command will move **Only** the images from the listed helm charts in the input files to the target **private** ECR in another account.

### Package Promotion

To promote a package from a private ECR to public you need the repository name. This repository must contain a Helm chart built by the process in the eks-anywhere-build-tooling git repository.
This will **both** for the helm chart, and the imgaes required to the public ECR of the calling user.

**This command only supports promotion of the latest helm chart from the private repository.**

```sh
generatebundlefile --promote hello-eks-anywhere
```

### Input File Supported Formats

Currently the following formats can be used as input files each of the following command flags.

--key alias/signingPackagesKey
--private-profile
--public--profile

#### Private Registry

```yaml
name: "v1-21-1001"
kubernetesVersion: "1.21"
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
name: "v1-21-1001"
kubernetesVersion: "1.21"
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

The "latest" tag will tell the program to use a timestamp lookup, and find the most recently pushed helm chart for information, even if it does have the actual latest tag.

```yaml
name: "v1-21-1001"
kubernetesVersion: "1.21"
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
name: "v1-21-1001"
kubernetesVersion: "1.21"
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
name: "v1-21-1001"
kubernetesVersion: "1.21"
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
name: "v1-21-1001"
kubernetesVersion: "1.21"
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
