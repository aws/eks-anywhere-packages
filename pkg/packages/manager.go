package packages

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/yaml"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/bundle"
	"github.com/aws/eks-anywhere-packages/pkg/driver"
	"github.com/aws/eks-anywhere-packages/pkg/utils"
)

const (
	retryNever     = time.Duration(0)
	retryNow       = time.Duration(1)
	retrySoon      = time.Duration(2) * time.Second
	retryShort     = time.Duration(30) * time.Second
	retryLong      = time.Duration(60) * time.Second
	retryVeryLong  = time.Duration(180) * time.Second
	sourceRegistry = "sourceRegistry"
)

type ManagerContext struct {
	Ctx           context.Context
	Package       api.Package
	PackageDriver driver.PackageDriver
	Source        api.PackageOCISource
	PBC           api.PackageBundleController
	Version       api.SourceVersion
	RequeueAfter  time.Duration
	Log           logr.Logger
	Bundle        *api.PackageBundle
	ManagerClient bundle.Client
}

func NewManagerContext(ctx context.Context, log logr.Logger, packageDriver driver.PackageDriver) *ManagerContext {
	return &ManagerContext{
		Ctx:           ctx,
		Log:           log,
		PackageDriver: packageDriver,
	}
}

func (mc *ManagerContext) SetUninstalling(namespace, name string) {
	mc.Package.Namespace = namespace
	mc.Package.Name = name
	mc.Package.Status.State = api.StateUninstalling
}

func (mc *ManagerContext) getImageRegistry(values map[string]interface{}) string {
	if val, ok := values[sourceRegistry]; ok {
		if val != "" {
			return val.(string)
		}
	}
	return mc.PBC.GetDefaultImageRegistry()
}

func processInitializing(mc *ManagerContext) bool {
	mc.Log.Info("New installation", "name", mc.Package.Name)
	mc.Package.Status.Source = mc.Source
	mc.Package.Status.State = api.StateInstallingDependencies
	mc.RequeueAfter = retryNow
	mc.Package.Spec.DeepCopyInto(&mc.Package.Status.Spec)
	return true
}

func processUpdating(mc *ManagerContext) bool {
	mc.Log.Info("Updating package ", "name", mc.Package.Name)
	mc.Package.Status.State = api.StateInstallingDependencies
	mc.RequeueAfter = retryNow
	return true
}

func processInstallingDependencies(mc *ManagerContext) bool {
	mc.Log.Info("Installing dependencies", "chart", mc.Source)
	dependencies, err := mc.Bundle.GetDependencies(mc.Version)
	if err != nil {
		mc.Package.Status.Detail = fmt.Sprintf(
			"invalid package bundle. (%s@%s bundle: %s)",
			mc.Package.Name,
			mc.Version,
			mc.Bundle.Name,
		)
		mc.Log.Info(mc.Package.Status.Detail)
		mc.RequeueAfter = retryLong
		return true
	}
	pkgs, err := mc.ManagerClient.GetPackageList(mc.Ctx, mc.Package.Namespace)
	if err != nil {
		mc.RequeueAfter = retryShort
		mc.Package.Status.Detail = err.Error()
		return true
	}
	pkgsNotReady := []api.Package{}

	for _, dep := range dependencies {
		var pkg *api.Package
		for i := range pkgs.Items {
			items := pkgs.Items
			if items[i].Spec.PackageName == dep.Name {
				pkg = &items[i]
			}
		}
		if pkg != nil {
			if pkg.Status.State != api.StateInstalled {
				pkgsNotReady = append(pkgsNotReady, *pkg)
			}
		} else {
			p := api.NewPackage(dep.Name, dep.Name, mc.Package.Namespace, mc.Package.Spec.Config)
			p.Spec.TargetNamespace = mc.Package.Spec.TargetNamespace
			pkgsNotReady = append(pkgsNotReady, p)
			err := mc.ManagerClient.CreatePackage(mc.Ctx, &p)
			if err != nil {
				mc.Log.Error(err, "creating dependency package")
			}
		}
	}

	if len(pkgsNotReady) > 0 {
		depsStr := utils.Map(pkgsNotReady, func(pkg api.Package) string { return pkg.Spec.PackageName })
		mc.Package.Status.Detail = "Waiting for dependencies: " + strings.Join(depsStr, ", ")
		mc.RequeueAfter = retrySoon
		return true
	}
	mc.Package.Status.State = api.StateInstalling
	mc.Package.Status.Detail = ""
	return true
}

func processInstalling(mc *ManagerContext) bool {
	mc.Package.Status.Source = mc.Source
	mc.Log.Info("installing/updating", "chart", mc.Source)
	var err error
	var values map[string]interface{}
	if values, err = mc.Package.GetValues(); err != nil {
		mc.Package.Status.Detail = err.Error()
		mc.Log.Error(err, "Install failed")
		return true
	}
	values[sourceRegistry] = mc.getImageRegistry(values)
	if mc.Source.Registry == "" {
		mc.Source.Registry = mc.PBC.GetDefaultRegistry()
	}
	if err := mc.PackageDriver.Initialize(mc.Ctx, mc.Package.GetClusterName()); err != nil {
		mc.Package.Status.Detail = err.Error()
		return true
	}

	createNamespace := mc.PBC.Spec.CreateNamespace
	if err := mc.PackageDriver.Install(mc.Ctx, mc.Package.Name, mc.Package.Spec.TargetNamespace, createNamespace, mc.Source, values); err != nil {
		mc.Package.Status.Detail = err.Error()
		mc.Log.Error(err, "Install failed")
		return true
	}
	mc.Log.Info("Installed", "name", mc.Package.Name, "chart", mc.Package.Status.Source)
	mc.Package.Status.State = api.StateInstalled
	mc.Package.Status.CurrentVersion = mc.Source.Version
	mc.Package.Status.Detail = ""
	if len(mc.Package.GetClusterName()) == 0 {
		mc.Package.Status.Detail = "Deprecated package namespace. Move to eksa-packages-" + os.Getenv("CLUSTER_NAME")
	}
	mc.Package.Spec.DeepCopyInto(&mc.Package.Status.Spec)
	return true
}

func processInstalled(mc *ManagerContext) bool {
	if mc.Package.Status.Source != mc.Source {
		mc.Package.Status.Source = mc.Source
		mc.Package.Status.State = api.StateUpdating
		mc.RequeueAfter = retryShort
		return true
	}
	var err error
	newValues := make(map[string]interface{})

	err = yaml.Unmarshal([]byte(mc.Package.Spec.Config), &newValues)
	if err != nil {
		mc.Log.Error(err, "unmarshaling current package configuration")
		mc.Package.Status.Detail = err.Error()
		mc.RequeueAfter = retryShort
		return true
	}

	if err := mc.PackageDriver.Initialize(mc.Ctx, mc.Package.GetClusterName()); err != nil {
		mc.Package.Status.Detail = err.Error()
		mc.Log.Error(err, "Initialization failed")
		return true
	}

	newValues[sourceRegistry] = mc.getImageRegistry(newValues)
	needs, err := mc.PackageDriver.IsConfigChanged(mc.Ctx, mc.Package.Name, newValues)
	if err != nil {
		mc.Log.Error(err, "checking necessity of reconfiguration")
		mc.Package.Status.Detail = err.Error()
		mc.RequeueAfter = retryLong
		return true
	}
	if needs {
		mc.Log.Info("configuration change detected, upgrading")
		mc.Package.Status.State = api.StateUpdating
		mc.RequeueAfter = retryShort
		mc.Package.Spec.DeepCopyInto(&mc.Package.Status.Spec)
		return true
	}
	mc.RequeueAfter = retryVeryLong

	return false
}

func processUninstalling(mc *ManagerContext) bool {
	if err := mc.PackageDriver.Initialize(mc.Ctx, mc.Package.GetClusterName()); err != nil {
		mc.Package.Status.Detail = err.Error()
		mc.Log.Error(err, "Initialization failed")
		return false
	}
	if err := mc.PackageDriver.Uninstall(mc.Ctx, mc.Package.Name); err != nil {
		mc.Package.Status.Detail = err.Error()
		mc.Log.Error(err, "Uninstall failed")
		return false
	}
	mc.Log.Info("Uninstalled", "name", mc.Package.Name)
	mc.Package.Status.Detail = ""
	mc.RequeueAfter = retryNever
	return false
}

func processUnknown(mc *ManagerContext) bool {
	mc.Log.Info("Unknown state", "name", mc.Package.Name)
	mc.Package.Status.Detail = "Unknown state: " + string(mc.Package.Status.State)
	mc.RequeueAfter = retryNever
	mc.Package.Spec.DeepCopyInto(&mc.Package.Status.Spec)
	return true
}

func processDone(mc *ManagerContext) bool {
	mc.RequeueAfter = retryNever
	return false
}

//go:generate mockgen -source manager.go -destination=mocks/manager.go -package=mocks Manager

type Manager interface {
	// Process package events returns true if status update
	Process(mc *ManagerContext) bool
}

type manager struct {
	packageStates map[api.StateEnum]func(*ManagerContext) bool
}

var (
	instance Manager
	once     sync.Once
)

func NewManager() Manager {
	once.Do(func() {
		instance = &(manager{
			packageStates: map[api.StateEnum]func(*ManagerContext) bool{
				api.StateInitializing:           processInitializing,
				api.StateInstalling:             processInstalling,
				api.StateInstallingDependencies: processInstallingDependencies,
				api.StateInstalled:              processInstalled,
				api.StateUpdating:               processUpdating,
				api.StateUninstalling:           processUninstalling,
				api.StateUnknown:                processDone,
			},
		})
	})
	return instance
}

func (m manager) getState(stateName api.StateEnum) func(*ManagerContext) bool {
	if stateName == "" {
		stateName = api.StateInitializing
	}
	if val, ok := m.packageStates[stateName]; ok {
		return val
	}
	return processUnknown
}

func (m manager) Process(mc *ManagerContext) bool {
	mc.RequeueAfter = retryLong
	if !mc.Package.IsValidNamespace() {
		mc.Package.Status.Detail = "Packages namespaces must start with: " + api.PackageNamespace
		mc.RequeueAfter = retryNever
		if mc.Package.Status.State == api.StateUnknown {
			return false
		}
		mc.Package.Status.State = api.StateUnknown
		return true
	}
	stateFunc := m.getState(mc.Package.Status.State)
	result := stateFunc(mc)
	if result {
		mc.Log.Info(
			"Updating",
			"namespace",
			mc.Package.Namespace,
			"name",
			mc.Package.Name,
			"state",
			mc.Package.Status.State,
			"chart",
			mc.Package.Status.Source,
		)
	}
	return result
}
