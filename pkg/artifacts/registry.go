package artifacts

import (
	"context"
	"path/filepath"

	"github.com/go-logr/logr"

	"github.com/aws/eks-anywhere-packages/pkg/registry"
)

const configPath = "/tmp/config/registry"

var certFile = filepath.Join(configPath, "ca.crt")

// RegistryPuller handles pulling OCI artifacts from an OCI registry
// (i.e. bundles)
type RegistryPuller struct {
	storageClient registry.StorageClient
	log           logr.Logger
}

var _ Puller = (*RegistryPuller)(nil)

// NewRegistryPuller creates and initializes a RegistryPuller.
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
	credentialStore.SetDirectory(configPath)
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
