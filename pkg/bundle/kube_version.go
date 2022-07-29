package bundle

import (
	"fmt"
	"strings"

	"k8s.io/client-go/discovery"
)

type KubeVersionClient interface {
	// ApiVersion returns the Kubernetes API version
	ApiVersion() (string, error)
}

type kubeVersionClient struct {
	kubeServerVersion discovery.ServerVersionInterface
}

func NewKubeVersionClient(serverVersion discovery.ServerVersionInterface) *kubeVersionClient {
	return &kubeVersionClient{
		kubeServerVersion: serverVersion,
	}
}

var _ KubeVersionClient = (*kubeVersionClient)(nil)

func (kvc *kubeVersionClient) ApiVersion() (string, error) {
	info, err := kvc.kubeServerVersion.ServerVersion()
	if err != nil {
		return "", fmt.Errorf("getting server version: %s", err)
	}
	version := fmt.Sprintf("v%s-%s", info.Major, info.Minor)
	// The minor version can have a trailing + character that we don't want.
	version = strings.ReplaceAll(version, "+", "")
	return version, nil
}
