package driver

import (
	"context"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

// PackageDriver is an interface for converting a CRD to a series of Kubernetes
// objects.
//
// Its first implementation will be Helm, but the interface is used to enhance
// and simplify testing as well as abstract the details of Helm.
type PackageDriver interface {
	// Initialize the package driver
	Initialize(ctx context.Context, clusterName string, namespace string) error

	// Install or upgrade an package.
	Install(ctx context.Context, name string, namespace string, source api.PackageOCISource, values map[string]interface{}) error

	// Uninstall an package.
	Uninstall(ctx context.Context, name string) error

	// IsConfigChanged indicates that the values passed differ from
	// those currently running.
	IsConfigChanged(ctx context.Context, name string,
		values map[string]interface{}) (bool, error)
}
