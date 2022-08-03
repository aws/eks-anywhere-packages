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
	AuthFilename() string

	// UpdateConfigMap Updates Config Map of namespaces with name of the installation
	UpdateConfigMap(ctx context.Context, name string, namespace string, add bool) error

	// GetSecretValues Retrieves ImagePullSecrets data to pass to helm chart
	GetSecretValues(ctx context.Context, namespace string) (map[string]interface{}, error)
}
