package packages

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/driver/mocks"
)

const packageName = "packageName"
const packageInstance = "packageInstance"
const originalConfiguration = `
make: willys
models:
  mb: "41"
  cj2a:
    year: "45"
`
const newConfiguration = `
make: willys
models:
  mc: "49"
  cj3a:
    year: "49"
`

type PackageOCISource = api.PackageOCISource

var expectedEmptySource = PackageOCISource{
	Registry:   "",
	Repository: "",
	Digest:     "",
}

var expectedSource = PackageOCISource{
	Registry:   "public.ecr.aws/j0a1m4z9/",
	Repository: "hello-eks-anywhere",
	Digest:     "sha256:f2ca1bb6c7e907d06dafe4687e579fce76b37e4e93b7605022da52e6ccc26fd2",
}

var expectedUpdate = PackageOCISource{
	Registry:   "public.ecr.aws/j0a1m4z9/",
	Repository: "hello-eks-anywhere",
	Digest:     "sha256:deadbeefc7e907d06dafe4687e579fce76b37e4e93b7605022da52e6ccc26fd2",
}

func givenPackage() api.Package {
	return api.Package{
		TypeMeta: metav1.TypeMeta{
			Kind: "Package",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      packageInstance,
			Namespace: "eksa-packages",
		},
		Spec: api.PackageSpec{
			PackageName: packageName,
			Config:      originalConfiguration,
		},
	}
}

func givenMockDriver(t *testing.T) *mocks.MockPackageDriver {
	gomockController := gomock.NewController(t)
	return mocks.NewMockPackageDriver(gomockController)
}

func givenMocks(t *testing.T) (*ManagerContext, *mocks.MockPackageDriver) {
	pkg := givenPackage()
	mockDriver := givenMockDriver(t)
	return &ManagerContext{
		Ctx:           context.Background(),
		Package:       pkg,
		PackageDriver: mockDriver,
		Source:        expectedSource,
		PBC: api.PackageBundleController{
			Spec: api.PackageBundleControllerSpec{
				PrivateRegistry: "privateRegistry",
			},
		},
		RequeueAfter: time.Duration(100),
		Log:          logr.Discard(),
	}, mockDriver
}

func thenManagerContext(t *testing.T, mc *ManagerContext, expectedState api.StateEnum, expectedSource PackageOCISource, expectedRequeue time.Duration, expectedDetail string) {
	assert.Equal(t, expectedState, mc.Package.Status.State)
	assert.Equal(t, expectedSource, mc.Package.Status.Source)
	assert.Equal(t, expectedRequeue, mc.RequeueAfter)
	assert.Equal(t, expectedDetail, mc.Package.Status.Detail)
}

func TestManagerContext_SetUninstalling(t *testing.T) {
	sut, _ := givenMocks(t)
	expectedName := "billy"
	expectedState := api.StateUninstalling

	sut.SetUninstalling(expectedName)

	assert.Equal(t, expectedName, sut.Package.Name)
	assert.Equal(t, expectedState, sut.Package.Status.State)
}

func TestManagerContext_getRegistry(t *testing.T) {
	t.Run("registry from values", func(t *testing.T) {
		sut, _ := givenMocks(t)
		values := make(map[string]interface{})
		values["sourceRegistry"] = "valuesRegistry"

		assert.Equal(t, "valuesRegistry", sut.getRegistry(values))
	})

	t.Run("registry from privateRegistry", func(t *testing.T) {
		sut, _ := givenMocks(t)
		values := make(map[string]interface{})

		assert.Equal(t, "privateRegistry", sut.getRegistry(values))
	})

	t.Run("registry from bundle package", func(t *testing.T) {
		sut, _ := givenMocks(t)
		values := make(map[string]interface{})
		sut.PBC.Spec.PrivateRegistry = ""

		assert.Equal(t, "public.ecr.aws/j0a1m4z9/", sut.getRegistry(values))
	})

	t.Run("registry from default gated registry", func(t *testing.T) {
		sut, _ := givenMocks(t)
		values := make(map[string]interface{})
		sut.PBC.Spec.PrivateRegistry = ""
		sut.Source.Registry = ""

		assert.Equal(t, "783794618700.dkr.ecr.us-west-2.amazonaws.com", sut.getRegistry(values))
	})
}

func TestNewManager(t *testing.T) {
	expectedManager := NewManager()
	actualManager := NewManager()
	assert.Equal(t, expectedManager, actualManager)
}

func TestManagerLifecycle(t *testing.T) {
	sut := NewManager()

	t.Run("New package added with no state initializing", func(t *testing.T) {
		mc, _ := givenMocks(t)
		mc.Package.Status.State = ""
		expectedRequeue := time.Duration(1)
		result := sut.Process(mc)
		assert.True(t, result)
		thenManagerContext(t, mc, api.StateInstalling, expectedSource, expectedRequeue, "")
	})

	t.Run("New package added with state initializing", func(t *testing.T) {
		mc, _ := givenMocks(t)
		mc.Package.Status.State = api.StateInitializing
		expectedRequeue := time.Duration(1)
		result := sut.Process(mc)
		assert.True(t, result)
		thenManagerContext(t, mc, api.StateInstalling, expectedSource, expectedRequeue, "")
	})

	t.Run("installing installs", func(t *testing.T) {
		mc, mockDriver := givenMocks(t)
		mc.Package.Status.State = api.StateInstalling
		mockDriver.EXPECT().Install(mc.Ctx, mc.Package.ObjectMeta.Name, mc.Package.Spec.TargetNamespace, mc.Source, gomock.Any()).Return(nil)
		result := sut.Process(mc)
		assert.True(t, result)
		thenManagerContext(t, mc, api.StateInstalled, expectedSource, 60*time.Second, "")
	})

	t.Run("installing install fails", func(t *testing.T) {
		mc, mockDriver := givenMocks(t)
		mc.Package.Status.State = api.StateInstalling
		mockDriver.EXPECT().Install(mc.Ctx, mc.Package.ObjectMeta.Name, mc.Package.Spec.TargetNamespace, mc.Source, gomock.Any()).Return(fmt.Errorf("boom"))
		result := sut.Process(mc)
		assert.True(t, result)
		thenManagerContext(t, mc, api.StateInstalling, expectedSource, 60*time.Second, "boom")
	})

	t.Run("installed upgrade triggered", func(t *testing.T) {
		mc, _ := givenMocks(t)
		mc.Package.Status.State = api.StateInstalled
		mc.Package.Status.Source = expectedSource
		mc.Source = expectedUpdate
		result := sut.Process(mc)
		assert.True(t, result)
		thenManagerContext(t, mc, api.StateUpdating, expectedUpdate, 30*time.Second, "")
	})

	t.Run("installed configuration update", func(t *testing.T) {
		mc, mockDriver := givenMocks(t)
		mc.Package.Status.State = api.StateInstalled
		mc.Package.Status.Source = expectedSource
		mc.Source = expectedSource
		mc.Package.Spec.Config = newConfiguration
		mockDriver.EXPECT().IsConfigChanged(mc.Ctx, mc.Package.Name, gomock.Any()).Return(true, nil)
		result := sut.Process(mc)
		assert.True(t, result)
		thenManagerContext(t, mc, api.StateUpdating, expectedSource, 30*time.Second, "")
	})

	t.Run("installed no configuration change", func(t *testing.T) {
		mc, mockDriver := givenMocks(t)
		mc.Package.Status.State = api.StateInstalled
		mc.Package.Status.Source = expectedSource
		mc.Source = expectedSource
		mc.Package.Spec.Config = originalConfiguration
		mockDriver.EXPECT().IsConfigChanged(mc.Ctx, mc.Package.Name, gomock.Any()).Return(false, nil)
		result := sut.Process(mc)
		assert.False(t, result)
		thenManagerContext(t, mc, api.StateInstalled, expectedSource, 180*time.Second, "")
	})

	t.Run("installed IsConfigChanged error", func(t *testing.T) {
		mc, mockDriver := givenMocks(t)
		mc.Package.Status.State = api.StateInstalled
		mc.Package.Status.Source = expectedSource
		mc.Source = expectedSource
		mc.Package.Spec.Config = newConfiguration
		mockDriver.EXPECT().IsConfigChanged(mc.Ctx, mc.Package.Name, gomock.Any()).Return(true, fmt.Errorf("boom"))
		result := sut.Process(mc)
		assert.True(t, result)
		thenManagerContext(t, mc, api.StateInstalled, expectedSource, 60*time.Second, "boom")
	})

	t.Run("installed configuration parse error", func(t *testing.T) {
		mc, _ := givenMocks(t)
		mc.Package.Status.State = api.StateInstalled
		mc.Package.Status.Source = expectedSource
		mc.Source = expectedSource
		mc.Package.Spec.Config = "bogus configuration ---- whatevs"
		result := sut.Process(mc)
		assert.True(t, result)
		thenManagerContext(t, mc, api.StateInstalled, expectedSource, 30*time.Second, "error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type map[string]interface {}")
	})

	t.Run("Uninstalling works", func(t *testing.T) {
		mc, mockDriver := givenMocks(t)
		mc.SetUninstalling(packageInstance)
		mockDriver.EXPECT().Uninstall(mc.Ctx, packageInstance).Return(nil)
		result := sut.Process(mc)
		assert.False(t, result)
		thenManagerContext(t, mc, api.StateUninstalling, expectedEmptySource, time.Duration(0), "")
	})

	t.Run("Uninstalling fails", func(t *testing.T) {
		mc, mockDriver := givenMocks(t)
		mc.SetUninstalling(packageInstance)
		mockDriver.EXPECT().Uninstall(mc.Ctx, packageInstance).Return(fmt.Errorf("crunch"))
		result := sut.Process(mc)
		assert.False(t, result)
		thenManagerContext(t, mc, api.StateUninstalling, expectedEmptySource, 60*time.Second, "crunch")
	})

	t.Run("Bogus state is reported", func(t *testing.T) {
		mc, _ := givenMocks(t)
		mc.Package.Status.State = "bogus"
		result := sut.Process(mc)
		assert.True(t, result)
		thenManagerContext(t, mc, "bogus", expectedEmptySource, time.Duration(0), "Unknown state: bogus")
	})

	t.Run("Unknown state is ignored", func(t *testing.T) {
		mc, _ := givenMocks(t)
		mc.Package.Status.State = api.StateUnknown
		result := sut.Process(mc)
		assert.False(t, result)
		thenManagerContext(t, mc, api.StateUnknown, expectedEmptySource, time.Duration(0), "")
	})

	t.Run("Package in wrong namespace should be ignored", func(t *testing.T) {
		mc, _ := givenMocks(t)
		mc.Package.Status.State = ""
		mc.Package.Namespace = "default"
		result := sut.Process(mc)
		assert.True(t, result)
		thenManagerContext(t, mc, api.StateUnknown, expectedSource, time.Duration(0), "Packages must be in namespace: eksa-packages")
	})

	t.Run("Package in wrong namespace should be ignored again", func(t *testing.T) {
		mc, _ := givenMocks(t)
		mc.Package.Status.State = api.StateUnknown
		mc.Package.Namespace = "default"
		result := sut.Process(mc)
		assert.False(t, result)
		thenManagerContext(t, mc, api.StateUnknown, expectedEmptySource, time.Duration(0), "")
	})
}
