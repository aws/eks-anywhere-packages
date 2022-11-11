
Several different container registries may be used by the package controller. There registries used for package installation, the location of package bundle, and for the package controller installation.

There are several possible ways to change registries during package installation. These are listed in the order of highest priority first:

1) The user can set `sourceRegistry` in the package custom resource `config` section.
2) The PackageBundleController resource for a cluster can have a the `privateRegistry` set.
3) The PackageBundleController resource for a cluster has two settings `defaultRegistry` and `defaultImageRegistry`.

The `privateRegistry` setting is for private registry support such as Harbor. This is the setting that should be used if the CLI download images and import images commands are used to populate the registry.  The `defaultRegistry` setting is used for the location of the helm charts and `defaultImageRegistry` is passed in to the helm chart as `sourceRegistry` for the container images associated with the chart. If harbor is used as a cache or proxy, the `defaultRegistry` and `defaultImageRegistry` settings should be used because the helm charts will be in one project and the images in another.  The `defaultRegistry` setting will default to "public.ecr.aws/eks-anywhere" and the `defaultImageRegistry` will default to "783794618700.dkr.ecr.us-west-2.amazonaws.com" if they are not set during controller installation.

The `defaultRegistry` from the cluster's PackageBundleController is used as the source registry for the package bundle.

The package controller is installed using a helm chart from a registry and that helm chart has a `sourceRegistry` value. The default value of `sourceRegistry` is public eks-anywhere ECR. If the package controller is installed from another registry, the `--set sourceRegistry=<otherRegistry>` should be set in the helm install command. During the helm install, you also may want to set `defaultRegistry`, `defaultImageRegistry` or `privateRegistry`.  To install out of the package controller using the staging build for example:

```
CLUSTER_NAME=${CLUSTER_NAME:-mack}
DEFAULT_REGISTRY="public.ecr.aws/l0g8r8j6"
DEFAULT_IMAGE_REGISTRY="857151390494.dkr.ecr.us-west-2.amazonaws.com"
set -x
helm install eks-anywhere-packages oci://${DEFAULT_REGISTRY}/eks-anywhere-packages \
    --version ${VERSION} \
    --set sourceRegistry=${DEFAULT_REGISTRY} \
    --set defaultRegistry=${DEFAULT_REGISTRY} \
    --set defaultImageRegistry=${DEFAULT_IMAGE_REGISTRY} \
    --set clusterName=${CLUSTER_NAME}
```

