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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/aws/modelrocket-add-ons/api/v1alpha1"
)

const packageControllerName = "PackageController"

// PackageControllerReconciler reconciles a PackageController object
type PackageControllerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func NewPackageControllerReconciler(client client.Client, scheme *runtime.Scheme, log logr.Logger) *PackageControllerReconciler {
	if log == nil {
		log = ctrl.Log.WithName(packageControllerName)
	}

	return &PackageControllerReconciler{
		Client: client,
		Scheme: scheme,
		Log:    log,
	}
}

func RegisterPackageControllerReconciler(mgr ctrl.Manager) (err error) {
	log := ctrl.Log.WithName(packageControllerName)
	reconciler := NewPackageControllerReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		log,
	)

	return ctrl.NewControllerManagedBy(mgr).
		For(&api.PackageController{}).
		Complete(reconciler)
}

//+kubebuilder:rbac:groups=packages.eks.amazonaws.com,resources=packagecontrollers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=packages.eks.amazonaws.com,resources=packagecontrollers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=packages.eks.amazonaws.com,resources=packagecontrollers/finalizers,verbs=update
// TODO: Add rbac for pod listing and whatever else we need

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PackageControllerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.Info("Reconcile:", "NamespacedName", req.NamespacedName)
	var pkgControllerConfig api.PackageController
	if err := r.Get(ctx, req.NamespacedName, &pkgControllerConfig); err != nil {
		r.Log.Info("Unable to fetch pkgControllerConfig", "request", req.NamespacedName)
		// TODO: Delete case
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	r.Log.Info("Fetched:", "pkgControllerConfig", pkgControllerConfig)

	// TODO: Add/update case
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PackageControllerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.PackageController{}).
		Complete(r)
}
