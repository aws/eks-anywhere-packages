# Remotely Managing a Cluster with the Package Controller

The Package Controller should be able to run on the management cluster and install, update and delete packages on a workload cluster. The package custom resources for workload cluster will exist on the management cluster. The package custom resources for workload cluster will exist on the management cluster.

# PackageBundleController (PBC) Custom Resource
The management cluster will have a PackageBundleController custom resource for each workload cluster. The name of the PBC will be the name of the workload cluster.  This will allow each cluster to have different active bundles, different Kubernetes versions, and potentially different source registries. For example, the PBC for the billy cluster:

```
apiVersion: packages.eks.amazonaws.com/v1alpha1
kind: PackageBundleController
metadata:
  name: billy
  namespace: eksa-packages
spec:
  logLevel: 4
  activeBundle: "v1-21-31"
  upgradeCheckInterval: "24h"
  source:
    registry: public.ecr.aws/eks-anywhere
    repository: eks-anywhere-packages-bundles
```
Currently, the PBC is in the eksa-packages namespace and must be named "eksa-packages-bundle-controller". This will have to be changed to support any cluster name in that namespace.

# PackageBundle (bundle) Custom Resource
We currently set the bundle state to show the user which bundle is active. Since we will have several clusters sharing the same set of bundles, we should remove this state and have the user look at the PBC to determine the active bundle. We may keep bundle state for invalid bundle names for instance.

# Package Custom Resource
A cluster specific namespace will need to be created for package custom resources in the form "eksa-packages-${cluster_name}". We use the name of the custom resource as the name of the installed package, so this will allow the same name across clusters. For example, to install the hello app on the billy cluster:

```
apiVersion: packages.eks.amazonaws.com/v1alpha1
kind: Package
metadata:
  name: prod-hello
  namespace: eksa-packages-billy
spec:
  packageName: hello-eks-anywhere
```

When a workload cluster is deleted, the namespace associated should be deleted.
# Helm Driver
The helm driver will need to be told where to install the package and it will need to set the kubeconfig for the target cluster. In the NewHelm function, we would have to set the `settings.KubeConfig` value to the location of the kubeconfig file. The easiest way to get the kubeconfig file would be to get it on the fly from the secret. CAPI creates a secret for each cluster with the administrative credentials and we can read this file and create a temporary file for the helm install command. There probably is some optimization here.

For example the tlhowe cluster is the management cluster and the billy cluster is the workload cluster:
```
 % k get secret -A | grep kubeconfig
eksa-system                         billy-kubeconfig                                  cluster.x-k8s.io/secret                1      9h
eksa-system                         tlhowe-kubeconfig                                 cluster.x-k8s.io/secret                1      3d6h
```

This [github isse](https://github.com/helm/helm/issues/6910) has some different ideas on how to change the authentication for the helm go client.

# Helm Chart
The helm chart will need to be changed to require a cluster name during installation. The chart will use the cluster name to create the PBC for the installation and the "eksa-packages-${cluster_name}" namespace for package resources.

# eks-anywhere CLI
Several changes are required in the CLI:
* pass cluster name to list packages
* pass cluster name to generate package
* during cluster creation create install Package Bundle Controller only if self managed
* during cluster creation create PackageBundleController custom resource
* possibly deprecate the package controller installation command
* fail the package controller installation if kubeconfig is pointing to a workload cluster
* during the package controller installation command create the PackageBundleController custom resource
* pass cluster name to imperative installation
* add new command to list PackageBundleControllers

