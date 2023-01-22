package registry_test

import (
	"testing"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry/remote/auth"

	"github.com/aws/eks-anywhere-packages/pkg/registry"
)

func TestCredentialStore(t *testing.T) {
	configFile := newConfigFile(t, "testdata")
	credentialStore := registry.NewCredentialStore(configFile)

	result, err := credentialStore.Credential("localhost")
	require.NoError(t, err)
	assertAuthEqual(t, auth.Credential{Username: "user", Password: "pass"}, result)

	result, err = credentialStore.Credential("harbor.eksa.demo:30003")
	require.NoError(t, err)
	assertAuthEqual(t, auth.Credential{Username: "captain", Password: "haddock"}, result)

	result, err = credentialStore.Credential("bogus")
	require.NoError(t, err)
	assertAuthEqual(t, auth.EmptyCredential, result)

	result, err = credentialStore.Credential("5551212.dkr.ecr.us-west-2.amazonaws.com")
	// This is a generic error, so using errors.Is won't work, and this is as
	// much of the string as we can reliably match against in a cross-platform
	// fashion. Until they change it, then everything will break.
	require.ErrorContains(t, err, "error getting credentials - err")
	assertAuthEqual(t, auth.EmptyCredential, result)
}

func TestCredentialStore_InitEmpty(t *testing.T) {
	registry.NewCredentialStore(newConfigFile(t, "testdata/empty"))
}

func newConfigFile(t *testing.T, dir string) *configfile.ConfigFile {
	t.Helper()
	configFile, err := config.Load(dir)
	require.NoError(t, err)
	return configFile
}

func assertAuthEqual(t *testing.T, expected, got auth.Credential) {
	t.Helper()
	assert.Equal(t, expected.Username, got.Username)
	assert.Equal(t, expected.Password, got.Password)
	assert.Equal(t, expected.AccessToken, got.AccessToken)
	assert.Equal(t, expected.RefreshToken, got.RefreshToken)
}
