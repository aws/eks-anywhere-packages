package artifacts

import "context"

//go:generate mockgen -source puller.go -destination=mocks/puller.go -package=mocks Client

// Puller is an interface to abstract interaction with OCI registries or other
// storage services.
type Puller interface {
	// Pull the artifact at the given reference.
	Pull(ctx context.Context, ref, clusterName string) ([]byte, error)
}
