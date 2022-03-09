# Generate Bundle Files

## How it works

This binary takes in an input file with different fields for curated packages we are supporting, and generates their corresponding Bundle CRD files.

Example Input file format

There are 3 types of helm version tags we can input 

1. Exact Name match, a tag named `v1.0.1.1345` will return the sha if there is exact name match.
2. A substring version with `-latest` at the end of the name. In this case we'll search for the most recent helm chart starting with `v0.1.1` and return that sha.
3. A tag of `latest` will return the last helm chart pushed to this repo, and it's tag and sha.

This will output 2 crd objects for the projects. One named after item in the project list.

## How to run

To build and run

```bash
#For Mac
make build

./generatebundlefile --input data/sample_input.yaml # To use a singular input file
./generatebundlefile # default input is all yaml files in STDIN, output is output/
```

To run for all .yaml files in stdin (except output/)
```bash
make run
```

To Run for a single file
```bash
go run . --input "data/input_120.yaml"
```

This will output all the corresponding CRD's in the `output` folder

## How to get images to run locally

First you must pull any corresponding images from the main ECR gallery repo's and push them into a new ECR gallery repo within your account. There is a simple bash script:

`hack/pullpush.sh` that shows how you would accomplish this on a single image.

## How to generate a sample bundle in correct format

```bash
go run . --generate-sample true
```

You will see the sample file at `output/1.21-bundle-crd.yaml`

## How to sign a file

We can use the input of a valid bundle file with the `--signature` command to annotate it with it's signature. This will fail if there is already another signature on the file.

```bash
go run . --input output/bundle-1.20.yaml --signature "signature-123"
```

```yaml
apiVersion: packages.eks.amazonaws.com/v1alpha1
kind: PackageBundle
metadata:
  creationTimestamp: null
  name: "v1-20-1001"
  namespace: eksa-packages
...
``` 
Becomes...

```yaml
apiVersion: packages.eks.amazonaws.com/v1alpha1
kind: PackageBundle
metadata:
  creationTimestamp: null
  name: "v1-20-1001"
  namespace: eksa-packages
  annotations:
    eksa.aws.com/signature: signature-123
...
```
