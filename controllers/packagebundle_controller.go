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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/artifacts"
	"github.com/aws/eks-anywhere-packages/pkg/authenticator"
	"github.com/aws/eks-anywhere-packages/pkg/bundle"
	"github.com/aws/eks-anywhere-packages/pkg/config"
)

const packageBundleName = "PackageBundle"

// PackageBundleReconciler reconciles a PackageBundle object
type PackageBundleReconciler struct {
	client.Client
	Log            logr.Logger
	Scheme         *runtime.Scheme
	bundleClient   bundle.Client
	registryClient bundle.RegistryClient
	bundleManager  bundle.Manager
}

func NewPackageBundleReconciler(client client.Client, scheme *runtime.Scheme,
	bundleClient bundle.Client, bundleManager bundle.Manager, registryClient bundle.RegistryClient, log logr.Logger) *PackageBundleReconciler {

	return &(PackageBundleReconciler{
		Client:         client,
		Scheme:         scheme,
		Log:            log,
		bundleClient:   bundleClient,
		registryClient: registryClient,
		bundleManager:  bundleManager,
	})
}

func RegisterPackageBundleReconciler(mgr ctrl.Manager) error {
	log := ctrl.Log.WithName(packageBundleName)
	bundleClient := bundle.NewManagerClient(mgr.GetClient())
	tcc := authenticator.NewTargetClusterClient(mgr.GetLogger(), mgr.GetConfig(), mgr.GetClient())
	puller := artifacts.NewRegistryPuller(log)
	registryClient := bundle.NewRegistryClient(puller)
	bundleManager := bundle.NewBundleManager(log, registryClient, bundleClient, tcc, config.GetGlobalConfig())
	r := NewPackageBundleReconciler(mgr.GetClient(), mgr.GetScheme(), bundleClient, bundleManager, registryClient, log)
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.PackageBundle{}).
		// Watch for changes in the PackageBundleController, and reconcile
		// bundles to update state when active bundle changes.
		Watches(&source.Kind{Type: &api.PackageBundleController{}},
			handler.EnqueueRequestsFromMapFunc(r.mapBundleReconcileRequests)).
		// Watch for creation or deletion of other bundles, so bundles can update
		// their states accordingly.
		Watches(&source.Kind{Type: &api.PackageBundle{}},
			handler.EnqueueRequestsFromMapFunc(r.mapBundleReconcileRequests),
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool { return true },
				DeleteFunc: func(e event.DeleteEvent) bool { return true },
			})).
		Complete(r)
}

//+kubebuilder:rbac:groups=packages.eks.amazonaws.com,resources=packagebundles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=packages.eks.amazonaws.com,resources=packagebundles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=packages.eks.amazonaws.com,resources=packagebundles/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the PackageBundle object against the actual cluster state, and then perform
// operations to make the cluster state reflect the state specified by the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *PackageBundleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.V(6).Info("Reconcile:", "bundle", req.NamespacedName)

	pkgBundle := &api.PackageBundle{}
	if err := r.Get(ctx, req.NamespacedName, pkgBundle); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}
		// ignore deletes
		return ctrl.Result{}, nil
	}

	r.Log.Info("Add/ProcessBundle:", "bundle", req.NamespacedName)
	change, err := r.bundleManager.ProcessBundle(ctx, pkgBundle)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("package bundle update: %s", err)
	}
	if change {
		err = r.Status().Update(ctx, pkgBundle)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// mapBundleReconcileRequests generates a reconcile Request for each package bundle in the system.
func (r *PackageBundleReconciler) mapBundleReconcileRequests(_ client.Object) (
	requests []reconcile.Request) {

	ctx := context.Background()
	bundles := &api.PackageBundleList{}
	err := r.List(ctx, bundles, &client.ListOptions{Namespace: api.PackageNamespace})
	if err != nil {
		r.Log.Error(err, "listing package bundles")
		return []reconcile.Request{}
	}

	requests = []reconcile.Request{}
	for _, bundle := range bundles.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      bundle.GetName(),
				Namespace: bundle.GetNamespace(),
			},
		})
	}

	return requests
}
