package artifacts

import "context"

// Puller is an interface to abstract interaction with OCI registries or other
// storage services.
type Puller interface {
	// Pull the artifact at the given reference.
	Pull(ctx context.Context, ref string) ([]byte, error)
}
