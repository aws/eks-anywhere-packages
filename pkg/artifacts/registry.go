package artifacts

import (
	"context"
	"github.com/aws/eks-anywhere-packages/pkg/registry"
)

// RegistryPuller handles pulling OCI artifacts from an OCI registry
// (i.e. bundles)
type RegistryPuller struct {
	storageClient registry.StorageClient
}

var _ Puller = (*RegistryPuller)(nil)

// NewRegistryPuller creates and initializes a RegistryPuller.
//
// It assumes AWS ECR, and uses a password that exists in the ECR_PASSWORD
// environment variable.
func NewRegistryPuller() *RegistryPuller {
	return &RegistryPuller{}
}

func (p *RegistryPuller) Pull(ctx context.Context, ref string) ([]byte, error) {
	var art registry.Artifact
	err := art.SetURI(ref)
	if err != nil {
		return nil, err
	}

	certificates, err := registry.GetCertificates("registry.crt")
	if err != nil {
		return nil, err
	}

	credentialStore := registry.NewCredentialStore()
	err = credentialStore.Init()
	if err != nil {
		return nil, err
	}

	sc := registry.NewStorageContext(art.Registry, credentialStore, certificates, false)
	p.storageClient = registry.NewOCIRegistry(sc)
	err = p.storageClient.Init()
	if err != nil {
		return nil, err
	}

	return registry.PullBytes(ctx, p.storageClient, art)
}
