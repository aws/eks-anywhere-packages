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

type PackageOCISource = api.PackageOCISource

func givenPackage() api.Package {
	config := map[string]string{"make": "willys", "models.mb": "41", "models.cj2a.year": "45"}
	return api.Package{
		TypeMeta: metav1.TypeMeta{
			Kind: "Package",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "bobby",
		},
		Spec: api.PackageSpec{Config: config},
	}
}

func givenMockDriver(t *testing.T) *mocks.MockPackageDriver {
	gomockController := gomock.NewController(t)
	return mocks.NewMockPackageDriver(gomockController)
}

func givenManagerContext(driver *mocks.MockPackageDriver) *ManagerContext {
	pkg := givenPackage()
	return &ManagerContext{
		Ctx:           context.Background(),
		Package:       pkg,
		PackageDriver: driver,
		Source: PackageOCISource{
			Registry:   "public.ecr.aws/j0a1m4z9/",
			Repository: "eks-anywhere-test",
			Tag:        "sha256:f2ca1bb6c7e907d06dafe4687e579fce76b37e4e93b7605022da52e6ccc26fd2",
		},
		RequeueAfter: time.Duration(100),
		Log:          logr.Discard(),
	}
}

func thenManagerContext(t *testing.T, mc *ManagerContext, expectedState api.StateEnum, expectedSource PackageOCISource, expectedRequeue time.Duration, expectedDetail string) {
	assert.Equal(t, expectedState, mc.Package.Status.State)
	assert.Equal(t, expectedSource, mc.Package.Status.Source)
	assert.Equal(t, expectedRequeue, mc.RequeueAfter)
	assert.Equal(t, expectedDetail, mc.Package.Status.Detail)
}

func TestManagerContext_SetUninstalling(t *testing.T) {
	sut := givenManagerContext(givenMockDriver(t))
	expectedName := "billy"
	expectedState := api.StateUninstalling

	sut.SetUninstalling(expectedName)

	if sut.Package.Name != expectedName {
		t.Errorf("expected <%s> actual <%s>", expectedName, sut.Package.Name)
	}
	if sut.Package.Status.State != expectedState {
		t.Errorf("expected <%s> actual <%s>", expectedState, sut.Package.Status.State)
	}
}

func TestNewManager(t *testing.T) {
	expectedManager := NewManager()
	actualManager := NewManager()
	if expectedManager != actualManager {
		t.Errorf("expected <%s> actual <%s>", expectedManager, actualManager)
	}
}

func TestManagerLifecycle(t *testing.T) {
	driver := givenMockDriver(t)
	mc := givenManagerContext(driver)
	sut := NewManager()
	expectedSource := PackageOCISource{
		Registry:   "public.ecr.aws/j0a1m4z9/",
		Repository: "eks-anywhere-test",
		Tag:        "sha256:f2ca1bb6c7e907d06dafe4687e579fce76b37e4e93b7605022da52e6ccc26fd2",
	}
	expectedUpdate := PackageOCISource{
		Registry:   "public.ecr.aws/j0a1m4z9/",
		Repository: "eks-anywhere-test",
		Tag:        "sha256:deadbeefc7e907d06dafe4687e579fce76b37e4e93b7605022da52e6ccc26fd2",
	}

	t.Run("Package set for install should trigger `Installing` state for the correct package", func(t *testing.T) {
		assert.Equal(t, api.StateEnum(""), mc.Package.Status.State)
		expectedRequeue := time.Duration(1)
		result := sut.Process(mc)
		assert.True(t, result)
		thenManagerContext(t, mc, api.StateInstalling, expectedSource, expectedRequeue, "")
	})

	t.Run("Packages in the `installing` state should proceed with installation", func(t *testing.T) {
		expectedRequeue := time.Duration(60 * time.Second)
		driver.EXPECT().Install(mc.Ctx, mc.Package.ObjectMeta.Name, mc.Package.Spec.TargetNamespace, mc.Source, gomock.Any())
		result := sut.Process(mc)
		assert.True(t, result)
		thenManagerContext(t, mc, api.StateInstalled, expectedSource, expectedRequeue, "")
	})

	t.Run("Successfully installed packages are re-checked every 180s", func(t *testing.T) {
		expectedRequeue := time.Duration(180 * time.Second)
		result := sut.Process(mc)
		assert.False(t, result)
		thenManagerContext(t, mc, api.StateInstalled, expectedSource, expectedRequeue, "")
	})

	t.Run("Updating the Source triggers an packages upgrade", func(t *testing.T) {
		expectedRequeue := time.Duration(30 * time.Second)
		mc.Source = expectedUpdate
		result := sut.Process(mc)
		assert.True(t, result)
		thenManagerContext(t, mc, api.StateUpdating, expectedUpdate, expectedRequeue, "")
	})

	t.Run("Update crashes, error is reported", func(t *testing.T) {
		expectedRequeue := time.Duration(60 * time.Second)
		driver.EXPECT().Install(mc.Ctx, mc.Package.ObjectMeta.Name, mc.Package.Spec.TargetNamespace, mc.Source, gomock.Any()).Return(fmt.Errorf("boom"))
		result := sut.Process(mc)
		assert.True(t, result)
		thenManagerContext(t, mc, api.StateUpdating, expectedUpdate, expectedRequeue, "boom")
	})

	t.Run("Update successful", func(t *testing.T) {
		expectedRequeue := time.Duration(60 * time.Second)
		driver.EXPECT().Install(mc.Ctx, mc.Package.ObjectMeta.Name, mc.Package.Spec.TargetNamespace, mc.Source, gomock.Any())
		result := sut.Process(mc)
		assert.True(t, result)
		thenManagerContext(t, mc, api.StateInstalled, expectedUpdate, expectedRequeue, "")
	})

	t.Run("Uninstalling crashes, error is reported", func(t *testing.T) {
		expectedRequeue := time.Duration(60 * time.Second)
		mc.SetUninstalling("bobby")
		driver.EXPECT().Uninstall(mc.Ctx, "bobby").Return(fmt.Errorf("crunch"))
		result := sut.Process(mc)
		assert.False(t, result)
		thenManagerContext(t, mc, api.StateUninstalling, expectedUpdate, expectedRequeue, "crunch")
	})

	t.Run("Uninstalling works, no error is reported", func(t *testing.T) {
		expectedRequeue := time.Duration(0)
		driver.EXPECT().Uninstall(mc.Ctx, "bobby")
		result := sut.Process(mc)
		assert.False(t, result)
		thenManagerContext(t, mc, api.StateUninstalling, expectedUpdate, expectedRequeue, "")
	})

	t.Run("Unknown state is reported", func(t *testing.T) {
		expectedRequeue := time.Duration(0)
		mc.Package.Status.State = api.StateUnknown
		result := sut.Process(mc)
		assert.True(t, result)
		thenManagerContext(t, mc, api.StateUnknown, expectedUpdate, expectedRequeue, "Unknown state: unknown")
	})
}
