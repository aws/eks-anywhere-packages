# Generate Bundle Files

## How it works

This binary takes in an input file with different fields for curated packages we are supporting, and generates their corresponding Bundle CRD files.

Example Input file format

```yaml
#input.yaml
kubernetesVersion: "1.21"
packages:
  - name: jetstack
    projects:
      - name: cert-manager-controller
        registry: public.ecr.aws/eks-anywhere
        repository: jetstack/cert-manager-controller
        versions:
            - name: v1.1.0-eks-a-4
            - name: v1.1.0-eks-a-3
  - name: cilium
    projects:
      - name: cilium
        registry: gallery.ecr.aws/isovalent
        repository: cilium
        versions:
            - name: v1.9.10-eksa-enterprise.1
```

This will output 2 crd objects for the projects. One named after item in the project list.

Here is one of the corresponding output crd files.

```yaml
apiversion: packages.eks.amazonaws.com/v1alpha1
kind: PackageBundle
metadata:
  name: cert-manager-controller
  namespace: eksa-packages
  annotations: {}
spec:
  name: cert-manager-controller
  packages:
  - name: cert-manager-controller
    source:
      registry: public.ecr.aws/eks-anywhere
      repository: jetstack/cert-manager-controller
      versions:
      - name: v1.1.0-eks-a-4
        digest: sha256:273fe866f82e7278a16ef4d32c5a4cb31b688aae48290080dd8f2f7f44485c5c
      - name: v1.1.0-eks-a-3
        digest: sha256:c3516d93fa52bdb459f46839d708c113d127895468ef6d6a86ec44003cc85c4d
  kubernetesVersion: "1.21"
status:
  upgradesavailable: []
```

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
go run . --input "bundle.yaml"
```

This will output all the corresponding CRD's in the `output` folder

## How to get images to run locally

First you must pull any corresponding images from the main ECR gallery repo's and push them into a new ECR gallery repo within your account. There is a simple bash script:

`hack/pullpush.sh` that shows how you would accomplish this on a single image.

## How to generate a sample bundle in correct format

```bash
go run . --generate-sample true
```

You will see the sample file at `output/1.21-bundle-crd.yaml` with the following content

```yaml
---
apiVersion: packages.eks.amazonaws.com/v1alpha1
kind: PackageBundle
metadata:
  creationTimestamp: null
  name: "1.21"
  namespace: eksa-packages
spec:
  packages:
  - name: sample-package
    source:
      registry: sample-Registry
      repository: sample-Repository
      versions:
      - name: v0.0
        digest: sha256:da25f5fdff88c259bb2ce7c0f1e9edddaf102dc4fb9cf5159ad6b902b5194e66
  kubeVersion: "1.21"
```
