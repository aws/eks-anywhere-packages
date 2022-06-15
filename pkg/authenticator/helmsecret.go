package authenticator

import (
	"os"
)

type helmsecret struct {
}

var _ Authenticator = (*helmsecret)(nil)

func NewHelmSecret() *helmsecret {
	return &helmsecret{}
}

func (s *helmsecret) GetAuthFileName() (string, error) {
	// Check if Helm registry config is set
	helmconfig := os.Getenv("HELM_REGISTRY_CONFIG")
	if helmconfig != "" {
		// Use HELM_REGISTRY_CONFIG
		return helmconfig, nil
	}

	// Check secret mount
	secretPath := "/secrets/.docker/config.json"
	_, err := os.Stat("/secrets/.docker/config.json")
	if os.IsNotExist(err){
		// No secret found return empty string indicating no private registry authentication
		return "", nil
	}

	return secretPath, nil
}
