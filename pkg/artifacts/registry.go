package artifacts

import (
	"context"
	"path/filepath"

	"github.com/docker/cli/cli/config"
	"github.com/go-logr/logr"

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
		p.log.Info("problem getting certificate file", "filename", certFile, "error", err.Error())
	}

	configFile, err := config.Load("")
	if err != nil {
		return nil, err
	}
	store := registry.NewDockerCredentialStore(configFile)

	sc := registry.NewStorageContext(art.Registry, store, certificates, false)
	client := registry.NewOCIRegistry(sc)
	err = client.Init()
	if err != nil {
		return nil, err
	}

	return registry.PullBytes(ctx, client, *art)
}
