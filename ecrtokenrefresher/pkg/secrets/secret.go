package secrets

import "k8s.io/client-go/kubernetes"

type Credential struct {
	Registry string
	Username string
	Password string
}

type RemoteClusterClientset map[string]*kubernetes.Clientset

type Secret interface {
	Init(defaultClientSet *kubernetes.Clientset, remoteClientSets RemoteClusterClientset) error
	IsActive() bool
	GetCredentials() ([]*Credential, error)
	BroadcastCredentials() error
}
