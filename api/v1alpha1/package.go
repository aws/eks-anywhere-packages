package v1alpha1

import (
	"strings"

	"sigs.k8s.io/yaml"
)

const (
	PackageKind      = "Package"
	PackageNamespace = "eksa-packages"
	namespacePrefix  = PackageNamespace + "-"
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

func (config *Package) GetClusterName() string {
	if strings.HasPrefix(config.Namespace, namespacePrefix) {
		clusterName := strings.TrimPrefix(config.Namespace, namespacePrefix)
		return clusterName
	}
	return ""
}

func (config *Package) IsValidNamespace() bool {
	if !strings.HasPrefix(config.Namespace, namespacePrefix) {
		if config.Namespace != PackageNamespace {
			return false
		}
	}
	return true
}
