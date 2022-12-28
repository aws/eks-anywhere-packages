package packages

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

//go:generate mockgen -source client.go -destination=mocks/client.go -package=mocks Client

type Client interface {
	// CreatePackage creates a package
	CreatePackage(ctx context.Context, pkg *api.Package) (err error)

	// GetPackageList retrieves the list of packages resources.
	GetPackageList(ctx context.Context, namespace string) (packages api.PackageList, err error)
}

type packageClient struct {
	client client.Client
}

func NewPackageClient(client client.Client) *packageClient {
	return &(packageClient{
		client: client,
	})
}

// CreatePackage Creates the given package resource
func (p *packageClient) CreatePackage(ctx context.Context, pkg *api.Package) (err error) {
	return p.client.Create(ctx, pkg)
}

// GetPackageList retrieves all packages present in the given namespace
func (p *packageClient) GetPackageList(ctx context.Context, namespace string) (packages api.PackageList, err error) {
	list := api.PackageList{}
	return list, p.client.List(ctx, &list, client.InNamespace(namespace))
}

var _ Client = (*packageClient)(nil)
