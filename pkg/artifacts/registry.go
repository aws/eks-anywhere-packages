package artifacts

import (
	"context"

	"github.com/go-logr/logr"

	"github.com/aws/eks-anywhere-packages/pkg/registry"
)

const certFile = "/tmp/registry-mirror/CACERTCONTENT"

// RegistryPuller handles pulling OCI artifacts from an OCI registry
// (i.e. bundles)
type RegistryPuller struct {
	storageClient registry.StorageClient
	log           logr.Logger
}

var _ Puller = (*RegistryPuller)(nil)

// NewRegistryPuller creates and initializes a RegistryPuller.
//
// It assumes AWS ECR, and uses a password that exists in the ECR_PASSWORD
// environment variable.
func NewRegistryPuller(logger logr.Logger) *RegistryPuller {
	return &RegistryPuller{
		log: logger,
	}
}

func (p *RegistryPuller) Pull(ctx context.Context, ref string) ([]byte, error) {
	var art registry.Artifact
	err := art.SetURI(ref)
	if err != nil {
		return nil, err
	}

	certificates, err := registry.GetCertificates(certFile)
	if err != nil {
		p.log.Error(err, "problem getting certificate file", "filename", certFile)
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
