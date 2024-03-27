package authenticator

import "context"

// DockerAuth Structure for the authentication file
type DockerAuth struct {
	Auths map[string]DockerAuthRegistry `json:"auths,omitempty"`
}

type DockerAuthRegistry struct {
	Auth string `json:"auth"`
}

//go:generate mockgen -source authenticator.go -destination=mocks/authenticator.go -package=mocks Authenticator

// Authenticator is an interface for creating an authentication file with credentials to private registries
//
// Currently this is used with the Helm Driver which takes credentials in this way
// For this first implementation, kubernetes secrets will be used to pass in a token
type Authenticator interface {
	// Initialize Points Authenticator to target cluster
	Initialize(clusterName string) error

	// AuthFilename Gets Authentication File Path for OCI Registry
	AuthFilename() string

	// AddToConfigMap Adds Namespace to config map
	AddToConfigMap(ctx context.Context, name, namespace string) error

	// DelFromConfigMap Removes Namespace from config map
	DelFromConfigMap(ctx context.Context, name, namespace string) error

	// GetSecretValues Retrieves ImagePullSecrets data to pass to helm chart
	GetSecretValues(ctx context.Context, namespace string) (map[string]interface{}, error)

	// AddSecretToAllNamespace Add Secrets to all namespaces
	AddSecretToAllNamespace(ctx context.Context) error
}
