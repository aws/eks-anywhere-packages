package v1alpha1

import (
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const (
	PackageKind       = "Package"
	PackageNamespace  = "eksa-packages"
	namespacePrefix   = PackageNamespace + "-"
	clusterNameEnvVar = "CLUSTER_NAME"
)

func (config *Package) MetaKind() string {
	return config.TypeMeta.Kind
}

func (config *Package) ExpectedKind() string {
	return PackageKind
}

func NewPackage(packageName string, name, namespace string) Package {
	return Package{
		TypeMeta: metav1.TypeMeta{
			Kind: PackageKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: PackageSpec{
			PackageName: packageName,
		},
	}
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

func (config *Package) IsOldNamespace() bool {
	return config.GetClusterName() == ""
}

func (config *Package) IsValidNamespace() bool {
	if !strings.HasPrefix(config.Namespace, namespacePrefix) {
		if config.Namespace != PackageNamespace {
			return false
		}
	}
	return true
}

// IsInstalledOnWorkload returns true if the package is being installed on a workload cluster
// returns false otherwise
func (config *Package) IsInstalledOnWorkload() bool {
	clusterName := config.GetClusterName()
	managementClusterName := os.Getenv(clusterNameEnvVar)

	return managementClusterName != clusterName
}
