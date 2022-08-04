package v1alpha1

const PackageBundleControllerKind = "PackageBundleController"

func (config *PackageBundleController) MetaKind() string {
	return config.TypeMeta.Kind
}

func (config *PackageBundleController) ExpectedKind() string {
	return PackageBundleControllerKind
}

func (config *PackageBundleController) IsIgnored() bool {
	return config.Name != PackageBundleControllerName || config.Namespace != PackageNamespace
}

func (s *PackageBundleControllerSource) GetRef() (baseRef string) {
	baseRef = s.Registry
	if s.Repository != "" {
		baseRef += "/" + s.Repository
	}

	return baseRef
}
