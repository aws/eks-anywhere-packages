package bundle

import (
	"testing"

	"github.com/aws/eks-anywhere-packages/pkg/testutil"
)

func TestApiVersion(t *testing.T) {
	t.Parallel()

	t.Run("golden path", func(t *testing.T) {
		t.Parallel()

		discovery := testutil.NewFakeDiscoveryWithDefaults()
		kvc := NewKubeVersionClient(discovery)

		got, err := kvc.ApiVersion()
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
		expected := "v1-21"
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("minor version+", func(t *testing.T) {
		t.Parallel()

		discovery := testutil.NewFakeDiscoveryWithDefaults()
		kvc := NewKubeVersionClient(discovery)

		got, err := kvc.ApiVersion()
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
		expected := "v1-21"
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		discovery := testutil.NewFakeDiscoveryWithDefaults()
		kvc := NewKubeVersionClient(discovery)

		got, err := kvc.ApiVersion()
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
		expected := "v1-21"
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})
}
