package authenticator

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthFilename(t *testing.T) {
	t.Parallel()

	t.Run("golden path for set HELM_REGISTRY_CONFIG", func(t *testing.T) {
		t.Parallel()

		testfile := "/test.txt"
		os.Setenv("HELM_REGISTRY_CONFIG", testfile)
		helmAuth := NewHelmSecret()
		val, err := helmAuth.AuthFilename()

		assert.Nil(t, err)
		assert.Equal(t, val, testfile)
	})

	t.Run("golden path for no config or secrets", func(t *testing.T) {
		t.Parallel()

		os.Setenv("HELM_REGISTRY_CONFIG", "")
		helmAuth := NewHelmSecret()
		val, err := helmAuth.AuthFilename()

		assert.Nil(t, err)
		assert.Equal(t, val, "")
	})
}
