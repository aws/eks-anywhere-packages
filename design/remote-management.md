# Remotely Managing a Cluster with the Package Controller

The Package Controller should be able to run on the management cluster and install, update and delete packages on a workload cluster. The package custom resources for workload cluster will exist on the management cluster.

# PackageBundleController (PBC) Custom Resource
The management cluster will have a PackageBundleController custom resource for each workload cluster. The name of the PBC will be the name of the workload cluster.  This will allow each cluster to have a different active bundle, differen Kubernetes version and potentially a different source registry. For example, the PBC for the billy cluster:

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
Currently, the PBC is in the eksa-packages namespace and must be named "eksa-packages-bundle-controller". This will have to be changed to support any name in that namespace.

# PackageBundle (bundle) Custom Resource
We currently set the bundle state to show the user which bundle is active. Since we will have several clusters sharing the same set of bundles, we should remove this state completely and have the user look at the PBC to determine the active bundle.

# Package Custom Resource
The package custom resource will need to be used so that the first element of the name indicates the target cluster. For example, if we were to install the hello application on the billy and the tlhowe cluster, we would have a `billy-my-hello` and `tlhowe-my-hello` and they would be installed on each cluster as the my-hello application.

```
apiVersion: packages.eks.amazonaws.com/v1alpha1
kind: Package
metadata:
  name: billy-my
  namespace: eksa-packages
spec:
  packageName: hello-eks-anywhere
```

# Helm Driver
The helm driver will need to be told where to install the package and it will need to set the kubeconfig for the target cluster. In the NewHelm function, we would have to set the `settings.KubeConfig` value to the location of the kubeconfig file. The easiest way to get the kubeconfig file would be to get it on the fly from the secret. CAPI creates a secret for each cluster with the administrative credentials and we can read this file and create a temporary file for the helm install command. There probably is some optimization here.

For example the tlhowe cluster is the management cluster and the billy cluster is the workload cluster:
```
 % k get secret -A | grep kubeconfig
eksa-system                         billy-kubeconfig                                  cluster.x-k8s.io/secret                1      9h
eksa-system                         tlhowe-kubeconfig                                 cluster.x-k8s.io/secret                1      3d6h
```
