package artifacts

import (
	"context"
	"fmt"
	"os"

	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"oras.land/oras-go/pkg/content"
	"oras.land/oras-go/pkg/oras"
)

// RegistryPuller handles pulling OCI artifacts from an OCI registry
// (i.e. bundles)
type RegistryPuller struct {
	resolver remotes.Resolver
	store    *content.Memory
}

var _ Puller = (*RegistryPuller)(nil)

// NewRegistryPuller creates and initializes a RegistryPuller.
//
// It assumes AWS ECR, and uses a password that exists in the ECR_PASSWORD
// environment variable.
func NewRegistryPuller() *RegistryPuller {
	authFn := func(hostname string) (string, string, error) {
		return "AWS", os.Getenv("ECR_PASSWORD"), nil
	}
	authorizer := docker.NewDockerAuthorizer(docker.WithAuthCreds(authFn))

	return &RegistryPuller{
		resolver: docker.NewResolver(docker.ResolverOptions{
			Hosts: docker.ConfigureDefaultRegistries(docker.WithAuthorizer(authorizer)),
		}),
		store: content.NewMemory(),
	}
}

func (p *RegistryPuller) Pull(ctx context.Context, ref string) ([]byte, error) {
	_, err := oras.Copy(ctx, p.resolver, ref, p.store, "")
	if err != nil {
		return nil, fmt.Errorf("pulling artifact %q: %s", ref, err)
	}

	_, data, ok := p.store.GetByName("bundle.yaml")
	if !ok {
		return nil, fmt.Errorf("getting bundle data: unknown")
	}

	return data, nil
}
