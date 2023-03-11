// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/artifacts"
	auth "github.com/aws/eks-anywhere-packages/pkg/authenticator"
	"github.com/aws/eks-anywhere-packages/pkg/bundle"
	"github.com/aws/eks-anywhere-packages/pkg/config"
	"github.com/aws/eks-anywhere-packages/pkg/driver"
	"github.com/aws/eks-anywhere-packages/pkg/packages"
)

const (
	packageName = "Package"
	retryLong   = time.Second * time.Duration(60)
)

// PackageReconciler reconciles a Package object
type PackageReconciler struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	PackageDriver driver.PackageDriver
	Manager       packages.Manager
	bundleManager bundle.Manager
	managerClient bundle.Client
}

func NewPackageReconciler(client client.Client, scheme *runtime.Scheme,
	driver driver.PackageDriver, manager packages.Manager,
	bundleManager bundle.Manager, managerClient bundle.Client,
	log logr.Logger) *PackageReconciler {

	return &PackageReconciler{
		Client:        client,
		Scheme:        scheme,
		PackageDriver: driver,
		Manager:       manager,
		bundleManager: bundleManager,
		managerClient: managerClient,
		Log:           log,
	}
}

func RegisterPackageReconciler(mgr ctrl.Manager) (err error) {
	log := ctrl.Log.WithName(packageName)
	manager := packages.NewManager()
	cfg := mgr.GetConfig()

	secretAuth, err := auth.NewECRSecret(cfg)
	if err != nil {
		return err
	}

	tcc := auth.NewTargetClusterClient(log, cfg, mgr.GetClient())
	helmDriver := driver.NewHelm(log, secretAuth, tcc)

	puller := artifacts.NewRegistryPuller(log)
	registryClient := bundle.NewRegistryClient(puller)
	managerClient := bundle.NewManagerClient(mgr.GetClient())
	bundleManager := bundle.NewBundleManager(log, registryClient, managerClient, tcc, config.GetGlobalConfig())
	reconciler := NewPackageReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		helmDriver,
		manager,
		bundleManager,
		managerClient,
		log,
	)

	return ctrl.NewControllerManagedBy(mgr).
		For(&api.Package{}).
		Watches(&source.Kind{Type: &api.PackageBundle{}},
			handler.EnqueueRequestsFromMapFunc(reconciler.mapBundleChangesToPackageUpdate)).
		Complete(reconciler)
}

func (r *PackageReconciler) mapBundleChangesToPackageUpdate(_ client.Object) (req []reconcile.Request) {
	ctx := context.Background()
	objs := &api.PackageList{}
	err := r.List(ctx, objs, &client.ListOptions{Namespace: api.PackageNamespace})
	if err != nil {
		return req
	}

	for _, o := range objs.Items {
		req = append(req, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: o.GetNamespace(),
				Name:      o.GetName(),
			}})
	}

	return req
}

//+kubebuilder:rbac:groups=packages.eks.amazonaws.com,resources=packages,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=packages.eks.amazonaws.com,resources=packages/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=packages.eks.amazonaws.com,resources=packages/finalizers,verbs=update
//+kubebuilder:rbac:groups="*",resources="*",verbs="*"

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PackageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.V(6).Info("Reconcile:", "NamespacedName", req.NamespacedName)
	managerContext := packages.NewManagerContext(ctx, r.Log, r.PackageDriver)
	managerContext.ManagerClient = r.managerClient

	// Get the CRD object from the k8s API.
	var err error
	if err = r.Get(ctx, req.NamespacedName, &managerContext.Package); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		managerContext.SetUninstalling(req.Namespace, req.Name)
	} else {
		pbc, err := r.managerClient.GetPackageBundleController(ctx, managerContext.Package.GetClusterName())
		if err != nil {
			r.Log.Error(err, "Getting package bundle controller")
			managerContext.Package.Status.Detail = err.Error()
			if err = r.Status().Update(ctx, &managerContext.Package); err != nil {
				return ctrl.Result{RequeueAfter: retryLong}, err
			}
			return ctrl.Result{RequeueAfter: retryLong}, nil
		}
		managerContext.PBC = *pbc

		bundle, err := r.managerClient.GetActiveBundle(ctx, managerContext.Package.GetClusterName())
		if err != nil {
			r.Log.Error(err, "Getting active bundle")
			managerContext.Package.Status.Detail = err.Error()
			if err = r.Status().Update(ctx, &managerContext.Package); err != nil {
				return ctrl.Result{RequeueAfter: retryLong}, err
			}
			return ctrl.Result{RequeueAfter: retryLong}, nil
		}
		managerContext.Bundle = bundle

		targetVersion := managerContext.Package.Spec.PackageVersion
		if targetVersion == "" {
			targetVersion = api.Latest
		}
		pkgName := managerContext.Package.Spec.PackageName
		pkg, err := bundle.FindPackage(pkgName)
		if err != nil {
			managerContext.Package.Status.Detail = fmt.Sprintf("Package %s is not in the active bundle (%s).", pkgName, bundle.Name)
			r.Log.Info(managerContext.Package.Status.Detail)
			if err = r.Status().Update(ctx, &managerContext.Package); err != nil {
				return ctrl.Result{RequeueAfter: managerContext.RequeueAfter}, err
			}
			return ctrl.Result{RequeueAfter: retryLong}, err
		}
		managerContext.Version, err = bundle.FindVersion(pkg, targetVersion)
		if err != nil {
			managerContext.Package.Status.Detail = fmt.Sprintf("Package %s@%s is not in the active bundle (%s).", pkgName, targetVersion, bundle.Name)
			r.Log.Info(managerContext.Package.Status.Detail)
			if err = r.Status().Update(ctx, &managerContext.Package); err != nil {
				return ctrl.Result{RequeueAfter: managerContext.RequeueAfter}, err
			}
			return ctrl.Result{RequeueAfter: retryLong}, err
		}
		managerContext.Source = bundle.GetOCISource(pkg, managerContext.Version)
		managerContext.Package.Status.TargetVersion = printableTargetVersion(managerContext.Source, targetVersion)
	}

	updateNeeded := r.Manager.Process(managerContext)
	if updateNeeded {
		r.Log.V(6).Info("Updating status", "namespace", managerContext.Package.Namespace, "name", managerContext.Package.Name, "state", managerContext.Package.Status.State)
		if err = r.Status().Update(ctx, &managerContext.Package); err != nil {
			return ctrl.Result{RequeueAfter: managerContext.RequeueAfter}, err
		}
	}

	return ctrl.Result{RequeueAfter: managerContext.RequeueAfter}, nil
}

func printableTargetVersion(source api.PackageOCISource, targetVersion string) string {
	ret := targetVersion
	if targetVersion == api.Latest {
		ret = fmt.Sprintf("%s (%s)", source.Version, targetVersion)
	}
	return ret
}

// SetupWithManager sets up the controller with the Manager.
func (r *PackageReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.Package{}).
		Complete(r)
}
