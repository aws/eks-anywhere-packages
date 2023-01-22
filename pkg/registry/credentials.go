package registry

import (
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/credentials"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// CredentialStore for registry credentials, like ~/.docker/config.json.
type CredentialStore struct {
	configFile *configfile.ConfigFile
}

// NewCredentialStore creates a CredentialStore.
func NewCredentialStore(configFile *configfile.ConfigFile) *CredentialStore {
	if !configFile.ContainsAuth() {
		configFile.CredentialsStore = credentials.DetectDefaultStore(configFile.CredentialsStore)
	}
	return &CredentialStore{
		configFile: configFile,
	}
}

// Credential get an authentication credential for a given registry.
func (cs *CredentialStore) Credential(registry string) (auth.Credential, error) {
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
