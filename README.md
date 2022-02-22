## Amazon EKS Anywhere Curated Packages

EKS Anywhere curated packages is a framework to manage installation, configuration and maintenance of components that provide general operational capabilities for Kubernetes applications.

### Getting Started

Create a cluster with EKS Anywhere and set and export with KUBECONFIG.

1. Install the CRDs:

        make install

1. For now, you need to login to helm to pull the images from public ECR:

        aws ecr-public get-login-password --region us-east-1 \
          | HELM_EXPERIMENTAL_OCI=1 helm registry login \
              --username AWS \
              --password-stdin public.ecr.aws

1. Run the controller locally:

        make run ENABLE_WEBHOOKS=false

1. Load the controller resources:

        kubectl apply -f api/testdata/packagecontroller.yaml
        kubectl apply -f api/testdata/bundlecontroller.yaml

1. Load a bundle resource:

        kubectl apply -f api/testdata/bundle_one.yaml

1. Create a package installation:

        kubectl apply -f api/testdata/test.yaml

1. Delete a package installation:

        kubectl delete package package-sample
