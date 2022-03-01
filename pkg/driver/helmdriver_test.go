package driver

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

func TestHelmChartURLIsPrefixed(t *testing.T) {
	t.Run("https yes", func(t *testing.T) {
		t.Parallel()
		if !helmChartURLIsPrefixed("https://foo") {
			t.Errorf("Expected true got false")
		}
	})

	t.Run("http yes", func(t *testing.T) {
		t.Parallel()
		if !helmChartURLIsPrefixed("http://foo") {
			t.Errorf("Expected true got false")
		}
	})

	t.Run("oci yes", func(t *testing.T) {
		t.Parallel()
		if !helmChartURLIsPrefixed("oci://foo") {
			t.Errorf("Expected true got false")
		}
	})

	t.Run("boo no", func(t *testing.T) {
		t.Parallel()
		if helmChartURLIsPrefixed("boo://foo") {
			t.Errorf("Expected false got true")
		}
	})
}

func TestNewHelm(t *testing.T) {
	helm, err := NewHelm(logr.Discard())
	assert.NoError(t, err)
	assert.NotNil(t, helm.log)
}

func TestPrefixName(t *testing.T) {
	d := &helmDriver{}

	expected := "eks-anywhere-test-foo"
	got := d.prefixName("foo")
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}
