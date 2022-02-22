package driver

import (
	"context"

	api "github.com/aws/modelrocket-add-ons/api/v1alpha1"
)

// PackageDriver is an interface for converting a CRD to a series of Kubernetes
// objects.
//
// Its first implementation will be Helm, but the interface is used to enhance
// and simplify testing as well as abstract the details of Helm.
type PackageDriver interface {
	// Install or upgrade an package.
	Install(ctx context.Context, name string, namespace string, source api.PackageOCISource, values map[string]interface{}) error

	// Uninstall an package.
	Uninstall(ctx context.Context, name string) error
}
