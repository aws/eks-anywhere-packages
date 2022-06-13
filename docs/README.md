## Amazon EKS Anywhere Curated Packages Documentation

---
***Preview and Pricing Disclaimer***

The EKS Anywhere package controller and the EKS Anywhere Curated Packages (referred to as “features”) are provided as “preview features” subject to the [AWS Service Terms](https://aws.amazon.com/service-terms/), (including Section 2 (Betas and Previews)) of the same. During the EKS Anywhere Curated Packages Public Preview, the AWS Service Terms are extended to provide customers access to these features free of charge. These features will be subject to a service charge and fee structure at ”General Availability“ of the features.

---

EKS Anywhere Curated Packages Documentation lives in this folder.

### Getting Started

Create a cluster with EKS Anywhere and set and export with KUBECONFIG.

1. Install the CRDs:

        make install

1. Run the controller locally:

        make run ENABLE_WEBHOOKS=false
        # If testing with private repositories
        make run ENABLE_WEBHOOKS=false OCI_CRED=~/.docker/config.json

1. Load the controller resources:

        kubectl apply -f api/testdata/packagecontroller.yaml
        kubectl apply -f api/testdata/packagebundlecontroller.yaml

1. Load a bundle resource:

        kubectl apply -f api/testdata/bundle_one.yaml

1. Create a package installation:

        kubectl apply -f api/testdata/test.yaml

1. Delete a package installation:

        kubectl delete package package-sample
