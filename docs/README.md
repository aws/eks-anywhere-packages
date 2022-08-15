## Amazon EKS Anywhere Curated Packages Documentation

---
The Amazon EKS Anywhere Curated Packages are only available to customers with the Amazon EKS Anywhere Enterprise Subscription. To request a free trial, talk to your Amazon representative or connect with one [here](https://aws.amazon.com/contact-us/sales-support-eks/).

---

EKS Anywhere Curated Packages Documentation lives in this folder.

### Getting Started

Create a cluster with EKS Anywhere and set and export with KUBECONFIG.

1. Install the CRDs:

        make install

1. Run the controller locally:

        make run ENABLE_WEBHOOKS=false
        # If testing with private repositories
        make run ENABLE_WEBHOOKS=false HELM_REGISTRY_CONFIG=~/.docker/config.json

1. Load the controller resources:

        kubectl apply -f api/testdata/packagebundlecontroller.yaml

1. Load a bundle resource:

        kubectl apply -f api/testdata/bundle_one.yaml

1. Create a package installation:

        kubectl apply -f api/testdata/test.yaml

1. Delete a package installation:

        kubectl delete package package-sample
