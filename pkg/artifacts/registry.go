package artifacts

import (
	"context"
	"github.com/go-logr/logr"
	"oras.land/oras-go/v2/registry/remote"

	"github.com/aws/eks-anywhere-packages/pkg/registry"
)

// RegistryPuller handles pulling OCI artifacts from an OCI registry
type RegistryPuller struct {
	log logr.Logger
}

var _ Puller = (*RegistryPuller)(nil)

// NewRegistryPuller creates and initializes a RegistryPuller.
func NewRegistryPuller(logger logr.Logger) *RegistryPuller {
	return &RegistryPuller{
		log: logger,
	}
}

func (p *RegistryPuller) Pull(ctx context.Context, ref string) ([]byte, error) {
	art, err := registry.ParseArtifactFromURI(ref)
	if err != nil {
		return nil, err
	}

	certificates, err := registry.GetManagementClusterCertificate()
	if err != nil {
		p.log.Info("problem getting certificate file", "error", err.Error())
	}

	configFile, err := registry.CredentialsConfigLoad()
	if err != nil {
		return nil, err
	}
	store := registry.NewDockerCredentialStore(configFile)

	sc := registry.NewStorageContext(art.Registry, store, certificates, false)
	remoteRegistry, err := remote.NewRegistry(art.Registry)
	if err != nil {
		return nil, err
	}
	client := registry.NewOCIRegistry(sc, remoteRegistry)

	return registry.PullBytes(ctx, client, *art)
}
