package bundle

import (
	"context"
	"fmt"
	"strings"
	"testing"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/testutil"
)

func TestDownloadBundle(t *testing.T) {
	t.Parallel()

	baseRef := "example.com/org"

	t.Run("golden path", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		puller := testutil.NewMockPuller()
		puller.WithFileData(t, "../../api/testdata/bundle_one.yaml")

		ecr := NewRegistryClient(puller)

		kubeVersion := "v1-21"
		tag := "latest"
		ref := fmt.Sprintf("%s:%s-%s", baseRef, kubeVersion, tag)

		bundle, err := ecr.DownloadBundle(ctx, ref)

		if err != nil {
			t.Fatalf("expected no error, got: %s", err)
		}

		if bundle == nil {
			t.Errorf("expected bundle to be non-nil")
		}

		if bundle != nil && len(bundle.Spec.Packages) != 3 {
			t.Errorf("expected three packages to be defined, found %d",
				len(bundle.Spec.Packages))
		}
		if bundle.Spec.Packages[0].Name != "test" {
			t.Errorf("expected first package name to be \"test\", got: %q",
				bundle.Spec.Packages[0].Name)
		}
		if bundle.Spec.Packages[1].Name != "flux" {
			t.Errorf("expected second package name to be \"flux\", got: %q",
				bundle.Spec.Packages[1].Name)
		}
		if bundle.Spec.Packages[2].Name != "harbor" {
			t.Errorf("expected third package name to be \"harbor\", got: %q",
				bundle.Spec.Packages[2].Name)
		}
	})

	t.Run("handles pull errors", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		puller := testutil.NewMockPuller()
		puller.WithError(fmt.Errorf("test error"))

		ecr := NewRegistryClient(puller)

		kubeVersion := "v1-21"
		tag := "latest"
		ref := fmt.Sprintf("%s:%s-%s", baseRef, kubeVersion, tag)

		_, err := ecr.DownloadBundle(ctx, ref)
		if err == nil {
			t.Errorf("expected error, got nil")
		}
	})

	t.Run("errors on empty responses", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		puller := testutil.NewMockPuller()
		puller.WithData([]byte(""))

		ecr := NewRegistryClient(puller)

		kubeVersion := "v1-21"
		tag := "latest"
		ref := fmt.Sprintf("%s:%s-%s", baseRef, kubeVersion, tag)

		_, err := ecr.DownloadBundle(ctx, ref)
		if err == nil {
			t.Errorf("expected error, got nil")
		}
	})

	t.Run("handles YAML unmarshalling errors", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		// stub oras.Pull
		puller := testutil.NewMockPuller()
		puller.WithData([]byte("invalid yaml"))

		ecr := NewRegistryClient(puller)

		kubeVersion := "v1-21"
		tag := "latest"
		ref := fmt.Sprintf("%s:%s-%s", baseRef, kubeVersion, tag)

		_, err := ecr.DownloadBundle(ctx, ref)
		if err == nil {
			t.Errorf("expected error, got nil")
		}
		// The k8s YAML library converts everything to JSON, so the error we'll
		// get will be a JSON one.
		if !strings.Contains(err.Error(), "JSON") {
			t.Errorf("expected YAML-related error, got: %s", err)
		}
	})
}

func TestBundleManager_LatestBundle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("latest bundle", func(t *testing.T) {
		puller := testutil.NewMockPuller()
		bundle := GivenBundle(api.PackageBundleStateAvailable)

		bundle.Namespace = "billy"
		bm := NewRegistryClient(puller)

		_, err := bm.LatestBundle(ctx, "test", "v1.21")

		if err == nil {
			t.Errorf("expected error, got nil")
		}
	})
}
