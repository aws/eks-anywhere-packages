package v1alpha1

import (
	"sigs.k8s.io/yaml"
)

const (
	PackageKind      = "Package"
	PackageNamespace = "eksa-packages"
)

func (config *Package) MetaKind() string {
	return config.TypeMeta.Kind
}

func (config *Package) ExpectedKind() string {
	return PackageKind
}

// GetValues convert spec values into generic values map
func (config *Package) GetValues() (values map[string]interface{}, err error) {
	mapInterfaces := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(config.Spec.Config), &mapInterfaces)
	return mapInterfaces, err
}
