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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/artifacts"
	"github.com/aws/eks-anywhere-packages/pkg/bundle"
)

const (
	packageBundleControllerName = "PackageBundleController"
)

// PackageBundleControllerReconciler reconciles a PackageBundleController object
type PackageBundleControllerReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	bundleManager bundle.Manager
	Log           logr.Logger
}

func NewPackageBundleControllerReconciler(client client.Client,
	scheme *runtime.Scheme, bundleManager bundle.Manager,
	log logr.Logger) *PackageBundleControllerReconciler {

	return &PackageBundleControllerReconciler{
		Client:        client,
		Scheme:        scheme,
		bundleManager: bundleManager,
		Log:           log,
	}
}

func RegisterPackageBundleControllerReconciler(mgr ctrl.Manager) error {
	log := ctrl.Log.WithName(packageBundleControllerName)
	cfg := mgr.GetConfig()
	discovery, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return fmt.Errorf("creating discovery client: %s", err)
	}
	puller := artifacts.NewRegistryPuller()
	bundleClient := bundle.NewPackageBundleClient(mgr.GetClient())
	bm := bundle.NewBundleManager(log, discovery, puller, bundleClient)
	reconciler := NewPackageBundleControllerReconciler(mgr.GetClient(),
		mgr.GetScheme(), bm, log)
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.PackageBundleController{}).
		Complete(reconciler)
}

//+kubebuilder:rbac:groups=packages.eks.amazonaws.com,resources=packagebundlecontrollers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=packages.eks.amazonaws.com,resources=packagebundlecontrollers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=packages.eks.amazonaws.com,resources=packagebundlecontrollers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the PackageBundleController object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *PackageBundleControllerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.V(6).Info("Reconcile:", "bundle", req.NamespacedName)

	result := ctrl.Result{
		Requeue:      true,
		RequeueAfter: DefaultUpgradeCheckInterval,
	}

	pbc := &api.PackageBundleController{}
	err := r.Client.Get(ctx, req.NamespacedName, pbc)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return result, fmt.Errorf("retrieving package bundle controller: %s", err)
		}
		r.Log.Info("Bundle controller deleted (ignoring)", "bundle controller", req.NamespacedName)
		return withoutRequeue(result), nil
	}
	result.RequeueAfter = pbc.Spec.UpgradeCheckInterval.Duration

	if pbc.IsIgnored() {
		if pbc.Status.State != api.BundleControllerStateIgnored {
			r.Log.V(6).Info("Ignoring", "PackageBundleController", req.NamespacedName)
			pbc.Status.State = api.BundleControllerStateIgnored
			err = r.Client.Status().Update(ctx, pbc, &client.UpdateOptions{})
			if err != nil {
				return withoutRequeue(result), fmt.Errorf("updating status to ignored: %s", err)
			}
		}
		return withoutRequeue(result), nil
	}

	latestBundle, err := r.bundleManager.LatestBundle(ctx, pbc.Spec.Source.BaseRef())
	if err != nil {
		r.Log.Error(err, "Unable to get latest bundle")
		if pbc.Status.State != api.BundleControllerStateDisconnected {
			r.Log.V(6).Info("Disconnected", "PackageBundleController", req.NamespacedName)
			pbc.Status.State = api.BundleControllerStateDisconnected
			err = r.Client.Status().Update(ctx, pbc, &client.UpdateOptions{})
			if err != nil {
				return result, fmt.Errorf("updating status to disconnected: %s", err)
			}
		}
		result.RequeueAfter = pbc.Spec.UpgradeCheckShortInterval.Duration
		return result, nil
	}

	if pbc.Status.State != api.BundleControllerStateActive {
		r.Log.V(6).Info("Activating", "PackageBundleController", req.NamespacedName)
		pbc.Status.State = api.BundleControllerStateActive
		err = r.Client.Status().Update(ctx, pbc, &client.UpdateOptions{})
		if err != nil {
			return result, fmt.Errorf("updating status: %s", err)
		}
	}

	err = r.bundleManager.ProcessLatestBundle(ctx, latestBundle)
	if err != nil {
		return result, fmt.Errorf("processing latest bundle: %s", err)
	}

	r.Log.V(6).Info("packagebundlecontroller reconciled")

	return result, nil
}

const DefaultUpgradeCheckInterval = time.Hour * 24

// SetupWithManager sets up the controller with the Manager.
func (r *PackageBundleControllerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.PackageBundleController{}).
		Complete(r)
}

func withoutRequeue(result ctrl.Result) ctrl.Result {
	result.Requeue = false
	return result
}
