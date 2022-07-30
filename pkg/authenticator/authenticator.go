package authenticator

import "context"

// DockerAuth Structure for the authentication file
type DockerAuth struct {
	Auths map[string]DockerAuthRegistry `json:"auths,omitempty"`
}

type DockerAuthRegistry struct {
	Auth string `json:"auth"`
}

// Authenticator is an interface for creating an authentication file with credentials to private registries
//
// Currently this is used with the Helm Driver which takes credentials in this way
// For this first implementation, kubernetes secrets will be used to pass in a token
type Authenticator interface {
	// AuthFilename Gets Authentication File Path for OCI Registry
	AuthFilename() (string, error)

	UpdateConfigMap(ctx context.Context, namespace string, add bool) error

	GetSecretValues(ctx context.Context) (map[string]interface{}, error)
}
