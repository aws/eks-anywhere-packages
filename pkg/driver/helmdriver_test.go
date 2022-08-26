package driver

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/kube"
	"helm.sh/helm/v3/pkg/release"
	fakerest "k8s.io/client-go/rest/fake"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	auth "github.com/aws/eks-anywhere-packages/pkg/authenticator"
)

func TestHelmChartURLIsPrefixed(t *testing.T) {
	t.Run("https yes", func(t *testing.T) {
		t.Parallel()
		assert.True(t, helmChartURLIsPrefixed("https://foo"))
	})

	t.Run("http yes", func(t *testing.T) {
		t.Parallel()
		assert.True(t, helmChartURLIsPrefixed("http://foo"))
	})

	t.Run("oci yes", func(t *testing.T) {
		t.Parallel()
		assert.True(t, helmChartURLIsPrefixed("oci://foo"))
	})

	t.Run("boo no", func(t *testing.T) {
		t.Parallel()
		assert.False(t, helmChartURLIsPrefixed("boo://foo"))
	})
}

func TestNewHelm(t *testing.T) {
	helm, err := createNewHelm(t)
	assert.NoError(t, err)
	assert.NotNil(t, helm.log)
}

func TestIsConfigChanged(t *testing.T) {
	t.Run("returns an error when the resource isn't found", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		values := map[string]interface{}{}
		helm, err := createNewHelm(t)
		require.NoError(t, err)
		helm.cfg.KubeClient = newMockKube(fmt.Errorf("blah"))

		_, err = helm.IsConfigChanged(ctx, "name-does-not-exist", values)

		assert.EqualError(t, err, "installation not found \"name-does-not-exist\": IsReachable test error blah")
	})

	t.Run("golden path returning true", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		const foo = 1
		origValues := map[string]interface{}{"foo": foo, "bar": true}
		newValues := shallowCopy(t, origValues)
		newValues["foo"] = foo + 1
		rel := &release.Release{Config: newValues}
		helm, err := createNewHelm(t)
		require.NoError(t, err)
		helm.cfg.KubeClient = newMockKube(nil)
		helm.cfg.Releases.Driver = newMockReleasesDriver(rel, nil)

		changed, err := helm.IsConfigChanged(ctx, "name-does-not-matter", origValues)
		if assert.NoError(t, err) {
			assert.True(t, changed)
		}
	})

	t.Run("golden path returning false", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		origValues := map[string]interface{}{"foo": 1, "bar": true}
		sameValues := shallowCopy(t, origValues)
		rel := &release.Release{Config: sameValues}
		helm, err := createNewHelm(t)
		require.NoError(t, err)
		helm.cfg.KubeClient = newMockKube(nil)
		helm.cfg.Releases.Driver = newMockReleasesDriver(rel, nil)

		changed, err := helm.IsConfigChanged(ctx, "name-does-not-matter", origValues)
		if assert.NoError(t, err) {
			assert.False(t, changed)
		}
	})

	t.Run("golden path returning false with imagePullSecret added", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		const foo = 1
		origValues := map[string]interface{}{"foo": foo, "bar": true}
		newValues := shallowCopy(t, origValues)
		newValues["imagePullSecrets"] = "test"
		rel := &release.Release{Config: newValues}
		helm, err := createNewHelm(t)
		require.NoError(t, err)
		helm.cfg.KubeClient = newMockKube(nil)
		helm.cfg.Releases.Driver = newMockReleasesDriver(rel, nil)

		changed, err := helm.IsConfigChanged(ctx, "name-does-not-matter", origValues)
		if assert.NoError(t, err) {
			assert.False(t, changed)
		}
	})

	t.Run("golden path returning true with imagePullSecret via config", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		const foo = 1
		origValues := map[string]interface{}{"foo": foo, "bar": true}
		newValues := shallowCopy(t, origValues)
		origValues["imagePullSecrets"] = "test"
		rel := &release.Release{Config: newValues}
		helm, err := createNewHelm(t)
		require.NoError(t, err)
		helm.cfg.KubeClient = newMockKube(nil)
		helm.cfg.Releases.Driver = newMockReleasesDriver(rel, nil)

		changed, err := helm.IsConfigChanged(ctx, "name-does-not-matter", origValues)
		if assert.NoError(t, err) {
			assert.True(t, changed)
		}
	})
}

// Helpers
func createNewHelm(t *testing.T) (*helmDriver, error) {
	fakeRestClient := fakerest.RESTClient{
		GroupVersion: api.GroupVersion,
	}
	secretAuth, err := auth.NewECRSecret(&fakeRestClient)
	require.NoError(t, err)
	helm, err := NewHelm(logr.Discard(), secretAuth)
	return helm, err
}

type mockKube struct {
	kube.Interface

	err error
}

func newMockKube(err error) *mockKube {
	return &mockKube{err: err}
}

func (k *mockKube) IsReachable() error {
	if k.err != nil {
		return fmt.Errorf("IsReachable test error %w", k.err)
	}
	return nil
}

type mockReleasesDriver struct {
	release *release.Release
	err     error
}

func newMockReleasesDriver(release *release.Release, err error) *mockReleasesDriver {
	return &mockReleasesDriver{
		release: release,
		err:     err,
	}
}

// generated via
// impl -dir $GOPATH/pkg/mod/helm.sh/helm/v3@v3.8.1/pkg/storage 'd *mockReleasesDriver' driver.Driver

func (d *mockReleasesDriver) Create(key string, rls *release.Release) error {
	panic("not implemented") // TODO: Implement
}

func (d *mockReleasesDriver) Update(key string, rls *release.Release) error {
	panic("not implemented") // TODO: Implement
}

func (d *mockReleasesDriver) Delete(key string) (*release.Release, error) {
	panic("not implemented") // TODO: Implement
}

func (d *mockReleasesDriver) Get(key string) (*release.Release, error) {
	if d.err != nil {
		return nil, d.err
	}

	return d.release, nil
}

func (d *mockReleasesDriver) List(filter func(*release.Release) bool) ([]*release.Release, error) {
	panic("not implemented") // TODO: Implement
}

func (d *mockReleasesDriver) Query(labels map[string]string) ([]*release.Release, error) {
	if d.err != nil {
		return nil, d.err
	}

	return []*release.Release{d.release}, nil
}

func (d *mockReleasesDriver) Name() string {
	panic("not implemented") // TODO: Implement
}

func shallowCopy(t *testing.T, src map[string]interface{}) map[string]interface{} {
	t.Helper()

	dst := make(map[string]interface{}, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
