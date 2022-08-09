package v1alpha1

import "path"

const (
	PackageBundleControllerKind = "PackageBundleController"
	PackageBundleControllerName = "eksa-packages-bundle-controller"
)

func (config *PackageBundleController) MetaKind() string {
	return config.TypeMeta.Kind
}

func (config *PackageBundleController) ExpectedKind() string {
	return PackageBundleControllerKind
}

func (config *PackageBundleController) IsIgnored() bool {
	return config.Namespace != PackageNamespace
}

func (config *PackageBundleController) GetDefaultRegistry() string {
	if config.Spec.DefaultRegistry != "" {
		return config.Spec.DefaultRegistry
	}
	return defaultRegistry
}

func (config *PackageBundleController) GetDefaultImageRegistry() string {
	if config.Spec.DefaultImageRegistry != "" {
		return config.Spec.DefaultImageRegistry
	}
	return defaultImageRegistry
}

func (config *PackageBundleController) GetBundleUri() (uri string) {
	return path.Join(config.GetDefaultRegistry(), config.Spec.BundleRepository)
}
