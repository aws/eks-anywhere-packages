package packages

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/yaml"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/driver"
)

const (
	retryNever     = time.Duration(0)
	retryNow       = time.Duration(1)
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
	Version       string
	RequeueAfter  time.Duration
	Log           logr.Logger
}

func (mc *ManagerContext) SetUninstalling(namespace string, name string) {
	mc.Package.Namespace = namespace
	mc.Package.Name = name
	mc.Package.Status.State = api.StateUninstalling
}

func (mc *ManagerContext) getRegistry(values map[string]interface{}) string {
	if val, ok := values[sourceRegistry]; ok {
		if val != "" {
			return val.(string)
		}
	}
	if mc.PBC.Spec.PrivateRegistry != "" {
		return mc.PBC.Spec.PrivateRegistry
	}
	if mc.Source.Registry != "" {
		return mc.Source.Registry
	}
	return mc.PBC.GetDefaultImageRegistry()
}

func processInitializing(mc *ManagerContext) bool {
	mc.Log.Info("New installation", "name", mc.Package.Name)
	mc.Package.Status.Source = mc.Source
	mc.Package.Status.State = api.StateInstalling
	mc.RequeueAfter = retryNow
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
	values[sourceRegistry] = mc.getRegistry(values)
	if mc.Source.Registry == "" {
		mc.Source.Registry = mc.PBC.GetDefaultRegistry()
	}
	if err := mc.PackageDriver.Initialize(mc.Ctx, mc.Package.GetClusterName(), mc.Package.Spec.TargetNamespace); err != nil {
		mc.Package.Status.Detail = err.Error()
		mc.Log.Error(err, "Initialization failed")
		return true
	}
	if err := mc.PackageDriver.Install(mc.Ctx, mc.Package.Name, mc.Package.Spec.TargetNamespace, mc.Source, values); err != nil {
		mc.Package.Status.Detail = err.Error()
		mc.Log.Error(err, "Install failed")
		return true
	}
	mc.Log.Info("Installed", "name", mc.Package.Name, "chart", mc.Package.Status.Source)
	mc.Package.Status.State = api.StateInstalled
	mc.Package.Status.CurrentVersion = mc.Source.Version
	mc.Package.Status.Detail = ""
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

	if err := mc.PackageDriver.Initialize(mc.Ctx, mc.Package.GetClusterName(), mc.Package.Spec.TargetNamespace); err != nil {
		mc.Package.Status.Detail = err.Error()
		mc.Log.Error(err, "Initialization failed")
		return true
	}

	newValues[sourceRegistry] = mc.getRegistry(newValues)
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
		return true
	}
	mc.RequeueAfter = retryVeryLong

	return false
}

func processUninstalling(mc *ManagerContext) bool {
	if err := mc.PackageDriver.Initialize(mc.Ctx, mc.Package.GetClusterName(), mc.Package.Spec.TargetNamespace); err != nil {
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
	return true
}

func processDone(mc *ManagerContext) bool {
	mc.RequeueAfter = retryNever
	return false
}

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
				api.StateInitializing: processInitializing,
				api.StateInstalling:   processInstalling,
				api.StateInstalled:    processInstalled,
				api.StateUpdating:     processInstalling,
				api.StateUninstalling: processUninstalling,
				api.StateUnknown:      processDone,
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
		mc.Log.Info("Updating", "namespace", mc.Package.Namespace, "name", mc.Package.Name, "state", mc.Package.Status.State, "chart", mc.Package.Status.Source)
	}
	return result
}
