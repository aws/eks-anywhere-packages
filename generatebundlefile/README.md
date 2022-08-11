# Generate Bundle Files

## How it works

This binary takes in an input file with different fields for curated packages we are supporting, and generates their corresponding Bundle CRD files. In addition it has the ability to promote all images, and helm chart between container registries.

Example Input file format

There are 3 types of helm version tags we can input 

1. Exact Name match, a tag named `v1.0.1.1345` will return the sha if there is exact name match.
2. A substring version with `-latest` at the end of the name. In this case we'll search for the most recent helm chart starting with `v0.1.1` and return that sha.
3. A tag of `latest` will return the last helm chart pushed to this repo, and it's tag and sha.

This will output 2 crd objects for the projects. One named after item in the project list.

## Bundle Creation

To run you need an AWS KMS key that is
- Asymmetric
- ECC_NIST_P256
- Key Policy enabled for CLI user to have `kms:Sign` configured. 

To build

```bash
#To Build
make build

# To Run
./generatebundlefile --input data/sample_input.yaml --key alias/signingPackagesKey
```

This will output all the corresponding CRD's into `output/bundle.yaml` 

## Promote
To Run Promote you need the name of ECR Private Repository you want to run it on. This has to contain a helm chart built by build-tooling.

```
go run . --promote hello-eks-anywhere
```
