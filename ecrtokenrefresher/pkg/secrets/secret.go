package secrets

import "k8s.io/client-go/kubernetes"

type Credential struct {
	Registry string
	Username string
	Password string
	CA       string
	Insecure string
}

type (
	ClusterClientSet  map[string]kubernetes.Interface
	ClusterCredential map[string][]*Credential
)

type Secret interface {
	Init(mgmtClusterName string, clientSets ClusterClientSet) error
	IsActive() bool
	GetClusterCredentials(clientSets ClusterClientSet) (ClusterCredential, error)
	BroadcastCredentials() error
	GetName() string
}
