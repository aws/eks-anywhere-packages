package packages

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/driver"
)

const ( // coordinate state names to enum values
	retryNever    = time.Duration(0)
	retryNow      = time.Duration(1)
	retryShort    = time.Duration(30) * time.Second
	retryLong     = time.Duration(60) * time.Second
	retryVeryLong = time.Duration(180) * time.Second
)

type ManagerContext struct {
	Ctx           context.Context
	Package       api.Package
	PackageDriver driver.PackageDriver
	Source        api.PackageOCISource
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
	if err := mc.PackageDriver.Install(mc.Ctx, mc.Package.Name, mc.Package.Spec.TargetNamespace, mc.Source, values); err != nil {
		mc.Package.Status.Detail = err.Error()
		mc.Log.Error(err, "Install failed")
		return true
	}
	mc.Log.Info("Installed", "name", mc.Package.Name, "chart", mc.Package.Status.Source)
	mc.Package.Status.State = api.StateInstalled
	mc.Package.Status.CurrentVersion = mc.Package.Spec.PackageVersion
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
