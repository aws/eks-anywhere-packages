package authenticator

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewHelmSecret(t *testing.T) {
	t.Parallel()

	t.Run("golden path", func(t *testing.T) {
		t.Parallel()

		helmAuth := NewHelmSecret()

		assert.NotNil(t, helmAuth)
	})
}

func TestGetAuthFileName(t *testing.T) {
	t.Parallel()

	t.Run("golden path for set HELM_REGISTRY_CONFIG", func(t *testing.T) {
		t.Parallel()

		testfile := "/test.txt"
		os.Setenv("HELM_REGISTRY_CONFIG", testfile)
		helmAuth := NewHelmSecret()
		val, err := helmAuth.GetAuthFileName()

		assert.Nil(t, err)
		assert.Equal(t, val, testfile)
	})

	t.Run("golden path for no config or secrets", func(t *testing.T) {
		t.Parallel()

		os.Setenv("HELM_REGISTRY_CONFIG", "")
		helmAuth := NewHelmSecret()
		val, err := helmAuth.GetAuthFileName()

		assert.Nil(t, err)
		assert.Equal(t, val, "")
	})
}
