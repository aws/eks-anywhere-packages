package artifacts

import (
	"context"
	"path/filepath"

	"github.com/docker/cli/cli/config"
	"github.com/go-logr/logr"
	"oras.land/oras-go/v2/registry/remote"

	"github.com/aws/eks-anywhere-packages/pkg/registry"
)

const configPath = "/tmp/config/registry"

var certFile = filepath.Join(configPath, "ca.crt")

// RegistryPuller handles pulling OCI artifacts from an OCI registry
// (i.e. bundles)
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

	certificates, err := registry.GetCertificates(certFile)
	if err != nil {
		p.log.Error(err, "problem getting certificate file", "filename", certFile)
	}

	configFile, err := config.Load("")
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
