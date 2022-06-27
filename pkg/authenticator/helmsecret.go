package authenticator

import (
	"os"
)

type helmSecret struct {
}

var _ Authenticator = (*helmSecret)(nil)

func NewHelmSecret() *helmSecret {
	return &helmSecret{}
}

func (s *helmSecret) AuthFilename() (string, error) {
	// Check if Helm registry config is set
	helmconfig := os.Getenv("HELM_REGISTRY_CONFIG")
	if helmconfig != "" {
		// Use HELM_REGISTRY_CONFIG
		return helmconfig, nil
	}

	return "", nil
}
