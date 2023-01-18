package registry

import (
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/credentials"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// DockerCredentialStore for Docker registry credentials, like ~/.docker/config.json.
type DockerCredentialStore struct {
	configFile *configfile.ConfigFile
}

// NewDockerCredentialStore creates a DockerCredentialStore.
func NewDockerCredentialStore(configFile *configfile.ConfigFile) *DockerCredentialStore {
	if !configFile.ContainsAuth() {
		configFile.CredentialsStore = credentials.DetectDefaultStore(configFile.CredentialsStore)
	}
	return &DockerCredentialStore{
		configFile: configFile,
	}
}

// Credential get an authentication credential for a given registry.
func (cs *DockerCredentialStore) Credential(registry string) (auth.Credential, error) {
	authConf, err := cs.configFile.GetCredentialsStore(registry).Get(registry)
	if err != nil {
		return auth.EmptyCredential, err
	}
	cred := auth.Credential{
		Username:     authConf.Username,
		Password:     authConf.Password,
		AccessToken:  authConf.RegistryToken,
		RefreshToken: authConf.IdentityToken,
	}
	return cred, nil
}
