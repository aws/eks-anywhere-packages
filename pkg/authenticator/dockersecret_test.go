package authenticator

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDockerSecret(t *testing.T) {
	t.Parallel()

	t.Run("golden path", func(t *testing.T) {
		t.Parallel()

		dockerAuth := NewDockerSecret()

		assert.NotNil(t, dockerAuth)
	})
}

func Test_createAuthFile(t *testing.T) {
	t.Parallel()

	t.Run("golden path", func(t *testing.T) {
		testString := "Test Config File"
		file, err := createAuthFile(testString)

		assert.Nil(t, err)
		assert.FileExists(t, file)
		assert.Equal(t, testString, getContentsOfFile(file))
	})
}

func Test_GetAuthFileName(t *testing.T) {
	t.Parallel()

	dockerAuth := NewDockerSecret()
	config := "some value here"
	os.Setenv("OCI_CRED", config)

	t.Run("golden path", func(t *testing.T) {
		authfile, err := dockerAuth.GetAuthFileName()

		assert.Nil(t, err)
		assert.FileExists(t, authfile)

		assert.Equal(t, config, getContentsOfFile(authfile))
	})
}

func Test_getSecretToken(t *testing.T) {
	t.Parallel()

	config := "some value here"
	os.Setenv("OCI_CRED", config)

	t.Run("golden path", func(t *testing.T) {
		secret, err := getSecretToken()

		assert.Nil(t, err)
		assert.Equal(t, secret, config)
	})
}

// Helpers
func getContentsOfFile(file string) string {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return ""
	}

	return string(content)
}
