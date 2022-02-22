package v1alpha1

const PackageBundleControllerKind = "PackageBundleController"

func (config *PackageBundleController) MetaKind() string {
	return config.TypeMeta.Kind
}

func (config *PackageBundleController) ExpectedKind() string {
	return PackageBundleControllerKind
}
