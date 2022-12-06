package artifacts

import (
	"context"
	"fmt"

	"oras.land/oras-go/pkg/content"
	"oras.land/oras-go/pkg/oras"
)

// RegistryPuller handles pulling OCI artifacts from an OCI registry
// (i.e. bundles)
type RegistryPuller struct{}

var _ Puller = (*RegistryPuller)(nil)

// NewRegistryPuller creates and initializes a RegistryPuller.
//
// It assumes AWS ECR, and uses a password that exists in the ECR_PASSWORD
// environment variable.
func NewRegistryPuller() *RegistryPuller {
	return &RegistryPuller{}
}

func (p *RegistryPuller) Pull(ctx context.Context, ref string) ([]byte, error) {
	registry, err := content.NewRegistry(content.RegistryOptions{Insecure: true})
	if err != nil {
		return nil, fmt.Errorf("creating registry: %w", err)
	}
	store := content.NewMemory()

	_, err = oras.Copy(ctx, registry, ref, store, "")
	if err != nil {
		return nil, fmt.Errorf("pulling artifact %q: %s", ref, err)
	}

	_, data, ok := store.GetByName("bundle.yaml")
	if !ok {
		return nil, fmt.Errorf("getting bundle data: unknown")
	}

	return data, nil
}
