package v1alpha1

const PackageControllerKind = "PackageController"

func (config *PackageController) MetaKind() string {
	return config.TypeMeta.Kind
}

func (config *PackageController) ExpectedKind() string {
	return PackageControllerKind
}
