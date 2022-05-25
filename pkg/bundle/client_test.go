package bundle

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)



func TestNewPackageBundleClient(t *testing.T) {
	t.Parallel()

	t.Run("golden path", func(t *testing.T) {
		t.Parallel()

		mockClient := newMockClient(nil)
		bundleClient := NewPackageBundleClient(mockClient)

		assert.NotNil(t, bundleClient)
	})
}

func TestBundleClient_IsActive(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("golden path returning true", func(t *testing.T) {
		mockClient := newMockClient(nil)
		bundleClient := NewPackageBundleClient(mockClient)
		bundle := givenPackageBundle(api.PackageBundleStateInactive)

		active, err := bundleClient.IsActive(ctx, bundle)

		assert.True(t, active)
		assert.Nil(t, err)
	})

	t.Run("error on failed get", func(t *testing.T) {
		mockClient := newMockClient(fmt.Errorf("Failed Get"))
		bundleClient := NewPackageBundleClient(mockClient)
		bundle := givenPackageBundle(api.PackageBundleStateInactive)

		_, err := bundleClient.IsActive(ctx, bundle)

		assert.NotNil(t, err)
	})

	t.Run("fails on wrong namespace", func(t *testing.T) {
		mockClient := newMockClient(nil)
		bundleClient := NewPackageBundleClient(mockClient)
		bundle := &api.PackageBundle{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "Wrong-Name",
			},
			Status: api.PackageBundleStatus{
				State: api.PackageBundleStateActive,
			},
		}

		active, err := bundleClient.IsActive(ctx, bundle)

		assert.False(t, active)
		assert.Nil(t, err)
	})

	t.Run("fails on wrong name", func(t *testing.T) {
		mockClient := newMockClient(nil)
		bundleClient := NewPackageBundleClient(mockClient)
		bundle := &api.PackageBundle{
			ObjectMeta: metav1.ObjectMeta{
				Name: "non-empty",
				Namespace: api.PackageNamespace,
			},
			Status: api.PackageBundleStatus{
				State: api.PackageBundleStateActive,
			},
		}

		active, err := bundleClient.IsActive(ctx, bundle)

		assert.False(t, active)
		assert.Nil(t, err)
	})
}

func TestBundleClient_GetActiveBundle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("golden path", func(t *testing.T) {
		mockClient := newMockClient(nil)
		bundleClient := NewPackageBundleClient(mockClient)

		bundle, err := bundleClient.GetActiveBundle(ctx)

		assert.Equal(t, bundle.Name, "test")
		assert.Nil(t, err)
	})
}

func TestBundleClient_GetActiveBundleNamespacedName(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("golden path", func(t *testing.T) {
		mockClient := newMockClient(nil)
		bundleClient := NewPackageBundleClient(mockClient)

		namespacedNames, err := bundleClient.GetActiveBundleNamespacedName(ctx)

		assert.Equal(t, api.PackageNamespace, namespacedNames.Namespace)
		assert.Equal(t, "", namespacedNames.Name)
		assert.Nil(t, err)
	})
}

// Helpers
type mockClient struct {
	client *client.Client
	err    error
}

func newMockClient(err error) *mockClient {
	return &mockClient{err: err}
}

// Helper Interfaces from Client
// generated via
// impl -dir $GOPATH/pkg/mod/sigs.k8s.io/controller-runtime\@v0.11.1/pkg/client 'c *mockClientDriver' Reader
// Get retrieves an obj for the given object key from the Kubernetes Cluster.
// obj must be a struct pointer so that obj can be updated with the response
// returned by the Server.

func (c *mockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	if c.err != nil {
		return c.err
	}

	obj.SetNamespace(api.PackageNamespace)
	obj.SetName("test")

	return nil
}

// List retrieves list of objects for a given namespace and list options. On a
// successful call, Items field in the list will be populated with the
// result returned from the server.
func (c *mockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	panic("not implemented") // TODO: Implement
}

// impl -dir $GOPATH/pkg/mod/sigs.k8s.io/controller-runtime\@v0.11.1/pkg/client 'c *mockClientDriver' Reader
// Create saves the object obj in the Kubernetes cluster.
func (c *mockClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	panic("not implemented") // TODO: Implement
}

// Delete deletes the given obj from Kubernetes cluster.
func (c *mockClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	panic("not implemented") // TODO: Implement
}

// Update updates the given obj in the Kubernetes cluster. obj must be a
// struct pointer so that obj can be updated with the content returned by the Server.
func (c *mockClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	panic("not implemented") // TODO: Implement
}

// Patch patches the given obj in the Kubernetes cluster. obj must be a
// struct pointer so that obj can be updated with the content returned by the Server.
func (c *mockClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	panic("not implemented") // TODO: Implement
}

// DeleteAllOf deletes all objects of the given type matching the given options.
func (c *mockClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	panic("not implemented") // TODO: Implement
}

func (c *mockClient) Status() client.StatusWriter {
	panic("not implemented") // TODO: Implement
}

func (c *mockClient) Scheme() *runtime.Scheme {
	panic("not implemented") // TODO: Implement
}

func (c *mockClient) RESTMapper() meta.RESTMapper {
	panic("not implemented") // TODO: Implement
}