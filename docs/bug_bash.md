

EKS Anywhere curated packages is a framework to manage installation, configuration and maintenance of components that provide general operational capabilities for Kubernetes applications. This guide describes the steps to install EKS Anywhere package controller and install, configure, upgrade and delete a package.

## Before you start

You must create a EKS Anywhere cluster The easiest thing to do is create a [local EKS Anywhere cluster](https://anywhere.eks.amazonaws.com/docs/getting-started/local-environment/) on your Mac or Linux machine using the Docker provider.

## Install package controller

Right now, EKS Anywhere Packages is installed using a helm chart. 

         helm install cert-manager oci://public.ecr.aws/j0a1m4z9/aws/eks-anywhere-packages --version v0.1.0

## Install eksctl-anywhere

There is not an official release that contains the latest CLI with all the features needed by EKS Anywhere Packages, so you will need to install a development release.

        curl http://bogus/artifacts/eksctl-anywhere -O
        chmod 0755 eksctl-anywhere
        mv eksctl-anywhere $(type -f eksctl-anywhere| cut -d' ' -f3)

## Listing the available packages

You can get a list of the packages that are available for install using

        eksctl anywhere list packages

## Install a package

The first step of installing a package is generating the package configuration

         eksctl anywhere generate package Harbor --name my-harbor > my-harbor.yaml

Create a certificate for your Harbor installation...

Add the certificate to the generated configuration. It should look something like

        apiVersion: packages.eks.amazonaws.com/v1alpha1
        kind: Package
        metadata:
          name: my-harbor
        spec:
          packageName: Harbor
          packageVersion: v2.4.1-c47074c1d3d92a2359f6b379e6688c323338ec18-helm
          config:
            certificate: ASDF

Install the package

         eksctl anywhere create package -f my-harbor.yaml

Make sure Harbor comes up by using a command like
         
         kubectl get pods --watch

## Connect to Harbor

You will need to use port forwarding to connect to Harbor:

        kubectl port-forward harbor-something

Once you have create the port forward, connect to it using the domain you create [https://my-harbor.local](https://my-harbor.local). Verify the version of Harbor installed is what you expected.

## Upgrade Harbor

In order to upgrade harbor, run the following command:

        eksctl anywhere upgrade package my-harbor v2.4.2-c47074c1d3d92a2359f6b379e6688c323338ec18-helm

Use the this command or something like it to monitor the upgrade

         kubectl get pods --watch

Once the pods are updated, verify you are running the correct version

## Validate Harbor works

Login to your private harbor

        docker login my-harbor.local

Validate Harbor works by logging in and creating a repository and pushing and image to it.

        docker pull nginx
        docker tag $(docker images nginx --format '{{.ID}}') my-harbor.local/nginx:latest
        docker push my-harbor.local/nginx:latest
        docker push 

## Delete Harbor

Remove your Harbor installation

        eksctl anywhere delete package my-harbor

Use your favorite command to watch it go away

        kubectl get pods --watch
