package bundle

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/testutil"
)

func TestLatestBundle(t *testing.T) {
	t.Parallel()

	baseRef := "example.com/org"
	discovery := testutil.NewFakeDiscoveryWithDefaults()

	t.Run("golden path", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		puller := testutil.NewMockPuller()
		puller.WithFileData(t, "../../api/testdata/bundle_one.yaml")

		bm := NewBundleManager(nil, discovery, puller)
		bundle, err := bm.LatestBundle(ctx, baseRef)
		if err != nil {
			t.Fatalf("expected no error, got: %s", err)
		}

		if bundle == nil {
			t.Errorf("expected bundle to be non-nil")
		}

		if bundle != nil && len(bundle.Spec.Packages) != 2 {
			t.Errorf("expected two packages to be defined, found %d",
				len(bundle.Spec.Packages))
		}
		if bundle.Spec.Packages[0].Name != "Test" {
			t.Errorf("expected first package name to be \"Test\", got: %q",
				bundle.Spec.Packages[0].Name)
		}
		if bundle.Spec.Packages[1].Name != "Flux" {
			t.Errorf("expected second package name to be \"Flux\", got: %q",
				bundle.Spec.Packages[1].Name)
		}
	})

	t.Run("handles pull errors", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		puller := testutil.NewMockPuller()
		puller.WithError(fmt.Errorf("test error"))

		bm := NewBundleManager(nil, discovery, puller)
		_, err := bm.LatestBundle(ctx, baseRef)
		if err == nil {
			t.Errorf("expected error, got nil")
		}
	})

	t.Run("errors on empty repsonses", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		puller := testutil.NewMockPuller()
		puller.WithData([]byte(""))

		bm := NewBundleManager(nil, discovery, puller)
		_, err := bm.LatestBundle(ctx, baseRef)
		if err == nil {
			t.Errorf("expected error, got nil")
		}
	})

	t.Run("handles YAML unmarshaling errors", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		// stub oras.Pull
		puller := testutil.NewMockPuller()
		puller.WithData([]byte("invalid yaml"))

		bm := NewBundleManager(nil, discovery, puller)
		_, err := bm.LatestBundle(ctx, baseRef)
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

func TestBuildNumber(t *testing.T) {
	t.Parallel()

	t.Run("golden path", func(t *testing.T) {
		t.Parallel()

		expected := 42
		if num, _ := buildNumber("v1.21-42"); num != expected {
			t.Errorf("expected %d, got %d", expected, num)
		}
	})

	t.Run("invalid int", func(t *testing.T) {
		t.Parallel()

		_, err := buildNumber("v1.21-abc")
		if err == nil {
			t.Errorf("expected error, got nil")
		}
	})
}

func TestKubeVersion(t *testing.T) {
	t.Parallel()

	t.Run("golden path", func(t *testing.T) {
		t.Parallel()

		expected := "v1.21"
		if ver, _ := kubeVersion("v1.21-42"); ver != expected {
			t.Errorf("expected %q, got %q", expected, ver)
		}
	})
}

func TestIsBundleOlderthan(t *testing.T) {
	t.Parallel()

	discovery := testutil.NewFakeDiscoveryWithDefaults()

	t.Run("golden path", func(t *testing.T) {
		t.Parallel()

		puller := testutil.NewMockPuller()
		bm := NewBundleManager(nil, discovery, puller)
		current := "v1.21-10002"
		candidate := "v1.21-10003"
		if older, _ := bm.IsBundleOlderThan(current, candidate); !older {
			t.Errorf("expected %q to be older than %q", current, candidate)
		}

		candidate = "v1.21-10001"
		if older, _ := bm.IsBundleOlderThan(current, candidate); older {
			t.Errorf("expected %q to be newer than %q", current, candidate)
		}
	})

	t.Run("newer kube version, older build is still true", func(t *testing.T) {
		t.Parallel()

		puller := testutil.NewMockPuller()
		bm := NewBundleManager(nil, discovery, puller)
		current := "v1.21-10002"
		candidate := "v1.22-10001"
		if older, _ := bm.IsBundleOlderThan(current, candidate); !older {
			t.Errorf("expected %q to be older than %q", current, candidate)
		}
	})

	t.Run("equal values returns false", func(t *testing.T) {
		t.Parallel()

		puller := testutil.NewMockPuller()
		bm := NewBundleManager(nil, discovery, puller)
		current := "v1.21-10002"
		candidate := "v1.21-10002"
		if older, _ := bm.IsBundleOlderThan(current, candidate); older {
			t.Errorf("expected %q not to be older than %q", current, candidate)
		}
	})
}

func TestPackageVersion(t *testing.T) {
	t.Parallel()

	t.Run("golden path", func(t *testing.T) {
		t.Parallel()

		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bm := NewBundleManager(nil, discovery, puller)

		got, err := bm.apiVersion()
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
		puller := testutil.NewMockPuller()
		bm := NewBundleManager(nil, discovery, puller)

		got, err := bm.apiVersion()
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
		expected := "v1-21"
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})
}

func TestUpdate(t *testing.T) {
	t.Parallel()

	noBundles := []api.PackageBundle{}

	t.Run("marks state active", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := &api.PackageBundle{
			Status: api.PackageBundleStatus{
				State: api.PackageBundleStateInactive,
			},
		}
		bm := NewBundleManager(nil, discovery, puller)

		if assert.True(t, bm.Update(bundle, true, noBundles)) {
			assert.Equal(t, api.PackageBundleStateActive, bundle.Status.State)
		}
	})

	t.Run("marks state inactive", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := &api.PackageBundle{
			Status: api.PackageBundleStatus{
				State: api.PackageBundleStateActive,
			},
		}
		bm := NewBundleManager(nil, discovery, puller)

		if assert.True(t, bm.Update(bundle, false, noBundles)) {
			assert.Equal(t, api.PackageBundleStateInactive, bundle.Status.State)
		}
	})

	t.Run("leaves state as-is (inactive)", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := &api.PackageBundle{
			Status: api.PackageBundleStatus{
				State: api.PackageBundleStateInactive,
			},
		}
		bm := NewBundleManager(nil, discovery, puller)

		if assert.True(t, bm.Update(bundle, false, noBundles)) {
			assert.Equal(t, api.PackageBundleStateInactive, bundle.Status.State)
		}
	})

	t.Run("leaves state as-is (active)", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := &api.PackageBundle{
			Status: api.PackageBundleStatus{
				State: api.PackageBundleStateActive,
			},
		}
		bm := NewBundleManager(nil, discovery, puller)

		if assert.True(t, bm.Update(bundle, true, noBundles)) {
			assert.Equal(t, api.PackageBundleStateActive, bundle.Status.State)
		}
	})

	t.Run("marks state upgrade available", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := api.MustPackageBundleFromFilename(t, "../../api/testdata/bundle_one.yaml")
		bundle.Status.State = api.PackageBundleStateActive
		allBundles := []api.PackageBundle{
			*bundle,
			*api.MustPackageBundleFromFilename(t, "../../api/testdata/bundle_two.yaml"),
		}
		bm := NewBundleManager(nil, discovery, puller)

		if assert.True(t, bm.Update(bundle, true, allBundles)) {
			assert.Equal(t, api.PackageBundleStateUpgradeAvailable, bundle.Status.State)
		}
	})

	t.Run("leaves state as-is (upgrade available)", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := api.MustPackageBundleFromFilename(t, "../../api/testdata/bundle_one.yaml")
		bundle.Status.State = api.PackageBundleStateUpgradeAvailable
		allBundles := []api.PackageBundle{
			*bundle,
			*api.MustPackageBundleFromFilename(t, "../../api/testdata/bundle_two.yaml"),
		}
		bm := NewBundleManager(nil, discovery, puller)

		if assert.True(t, bm.Update(bundle, true, allBundles)) {
			assert.Equal(t, api.PackageBundleStateUpgradeAvailable, bundle.Status.State)
		}
	})
}

func TestSortBundleNewestFirst(t *testing.T) {
	t.Run("it sorts newest version first", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := &api.PackageBundle{
			Status: api.PackageBundleStatus{
				State: api.PackageBundleStateActive,
			},
		}
		allBundles := []api.PackageBundle{
			*api.MustPackageBundleFromFilename(t, "../../api/testdata/bundle_one.yaml"),
			*api.MustPackageBundleFromFilename(t, "../../api/testdata/bundle_two.yaml"),
		}

		bm := NewBundleManager(log.NullLogger{}, discovery, puller)
		_ = bm.Update(bundle, true, allBundles)
		if assert.Greater(t, len(allBundles), 1) {
			assert.Equal(t, "v1-21-1002", allBundles[0].Name)
			assert.Equal(t, "v1-21-1001", allBundles[1].Name)
		}
	})

	t.Run("invalid names go to the end", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := &api.PackageBundle{
			Status: api.PackageBundleStatus{
				State: api.PackageBundleStateActive,
			},
		}
		allBundles := []api.PackageBundle{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "funky",
				},
				Status: api.PackageBundleStatus{
					State: api.PackageBundleStateInactive,
				},
			},
			*api.MustPackageBundleFromFilename(t, "../../api/testdata/bundle_one.yaml"),
			*api.MustPackageBundleFromFilename(t, "../../api/testdata/bundle_two.yaml"),
		}

		bm := NewBundleManager(log.NullLogger{}, discovery, puller)
		_ = bm.Update(bundle, true, allBundles)
		if assert.Greater(t, len(allBundles), 2) {
			assert.Equal(t, "funky", allBundles[2].Name)
		}
	})
}
