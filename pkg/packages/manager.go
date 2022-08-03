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
	Registry      string
	Version       string
	RequeueAfter  time.Duration
	Log           logr.Logger
}

func (mc *ManagerContext) SetUninstalling(name string) {
	mc.Package.Name = name
	mc.Package.Status.State = api.StateUninstalling
}

func processInitializing(mc *ManagerContext) bool {
	mc.Log.Info("New installation", "name", mc.Package.Name)
	mc.Package.Status.Source = mc.Source
	if mc.Package.Namespace != api.PackageNamespace {
		mc.Package.Status.State = api.StateUnknown
		mc.Package.Status.Detail = "Packages must be in namespace: " + api.PackageNamespace
		mc.RequeueAfter = retryNever
		return true
	}
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
	if mc.Registry != "" && values[sourceRegistry] == "" {
		values[sourceRegistry] = mc.Registry
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
	} else {
		var err error
		newValues := make(map[string]interface{})

		err = yaml.Unmarshal([]byte(mc.Package.Spec.Config), &newValues)
		if err != nil {
			mc.Log.Error(err, "unmarshaling current package configuration")
			mc.Package.Status.Detail = err.Error()
			mc.RequeueAfter = retryShort
			return true
		}

		if mc.Registry != "" && newValues[sourceRegistry] == "" {
			newValues[sourceRegistry] = mc.Registry
		}
		needs, err := mc.PackageDriver.IsConfigChanged(mc.Ctx, mc.Package.Name,
			newValues)
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
	}
	mc.RequeueAfter = retryVeryLong

	return false
}

func processUninstalling(mc *ManagerContext) bool {
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
	stateFunc := m.getState(mc.Package.Status.State)
	result := stateFunc(mc)
	if result {
		mc.Log.Info("Updating", "name", mc.Package.Name, "state", mc.Package.Status.State, "chart", mc.Package.Status.Source)
	}
	return result
}
