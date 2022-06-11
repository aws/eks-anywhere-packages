package authenticator

import (
	"fmt"
	"os"
)

type dockersecret struct {
}

var _ Authenticator = (*dockersecret)(nil)

func NewDockerSecret() *dockersecret {
	return &dockersecret{}
}

func (s *dockersecret) GetAuthFile() (string, error) {
	token, err := getSecretToken()
	if err != nil {
		return "", fmt.Errorf("Failed to get authfile %s\n", err)
	}
	authfile, err := createAuthFile(token)
	if err != nil {
		return "", fmt.Errorf("Failed to get authfile %s\n", err)
	}

	return authfile, nil
}

func getSecretToken() (string, error) {
	// TODO Handle encryption here if secret is encrypted
	dockerconfig := os.Getenv("DOCKER_CONFIG")

	return dockerconfig, nil
}

func createAuthFile(data string) (string, error) {
	f, err := os.CreateTemp("", "dockerAuth")
	if err != nil {
		return "", fmt.Errorf("Creating tempfile %w", err)
	}
	defer f.Close()
	fmt.Fprint(f, data)
	return f.Name(), nil
}
