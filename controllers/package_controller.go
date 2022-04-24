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
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/artifacts"
	"github.com/aws/eks-anywhere-packages/pkg/bundle"
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
}

func NewPackageReconciler(client client.Client, scheme *runtime.Scheme,
	driver driver.PackageDriver, manager packages.Manager,
	bundleManager bundle.Manager, log logr.Logger) *PackageReconciler {

	return &PackageReconciler{
		Client:        client,
		Scheme:        scheme,
		PackageDriver: driver,
		Manager:       manager,
		bundleManager: bundleManager,
		Log:           log.WithName(packageName),
	}
}

func RegisterPackageReconciler(mgr ctrl.Manager) (err error) {
	log := ctrl.Log.WithName(packageName)
	helmDriver, err := driver.NewHelm(log)
	if err != nil {
		return fmt.Errorf("creating helm driver: %w", err)
	}
	manager := packages.NewManager()
	cfg := mgr.GetConfig()
	discovery, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return fmt.Errorf("creating discovery client: %s", err)
	}
	puller := artifacts.NewRegistryPuller()
	bundleManager := bundle.NewBundleManager(log, discovery, puller)
	reconciler := NewPackageReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		helmDriver,
		manager,
		bundleManager,
		log,
	)

	return ctrl.NewControllerManagedBy(mgr).
		For(&api.Package{}).
		Watches(&source.Kind{Type: &api.PackageBundle{}},
			handler.EnqueueRequestsFromMapFunc(reconciler.mapBundleChangesToPackageUpdate)).
		Complete(reconciler)
}

func (r *PackageReconciler) mapBundleChangesToPackageUpdate(obj client.Object) (req []reconcile.Request) {
	ctx := context.Background()
	objs := &api.PackageList{}
	err := r.List(ctx, objs)
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

	// Get the CRD object from the k8s API.
	var err error
	managerContext := &packages.ManagerContext{
		Ctx:           ctx,
		Log:           r.Log,
		PackageDriver: r.PackageDriver,
	}

	if err = r.Get(ctx, req.NamespacedName, &managerContext.Package); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		if req.Namespace == api.PackageNamespace {
			managerContext.SetUninstalling(req.Name)
		}
	} else {
		bundle, err := r.bundleManager.ActiveBundle(ctx, r.Client)
		if err != nil {
			return ctrl.Result{RequeueAfter: retryLong}, err
		}

		targetVersion := managerContext.Package.Spec.PackageVersion
		if targetVersion == "" {
			targetVersion = api.Latest
		}
		pkgName := managerContext.Package.Spec.PackageName
		managerContext.Source, err = bundle.FindSource(pkgName, targetVersion)
		managerContext.Package.Status.TargetVersion = printableTargetVersion(managerContext.Source, targetVersion)
		if err != nil {
			//TODO add a link to documentation on how to make a bundle active.
			managerContext.Package.Status.Detail = fmt.Sprintf("Package %s@%s is not in the active bundle (%s). Did you forget to activate the new bundle?", pkgName, targetVersion, bundle.ObjectMeta.Name)
			if err = r.Status().Update(ctx, &managerContext.Package); err != nil {
				return ctrl.Result{RequeueAfter: managerContext.RequeueAfter}, err
			}
			return ctrl.Result{RequeueAfter: retryLong}, err
		}
	}

	updateNeeded := r.Manager.Process(managerContext)
	if updateNeeded {
		r.Log.V(6).Info("Updating status....")
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
