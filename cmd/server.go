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

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/controllers"
	"github.com/aws/eks-anywhere-packages/pkg/webhook"
)

var scheme = runtime.NewScheme()

type serverContext struct {
	metricsAddr          string
	enableLeaderElection bool
	probeAddr            string
}

var serverCommandContext = &serverContext{}

func init() {
	ctrl.Log.WithName("server")
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))

	rootCmd.AddCommand(serverCommand)

	serverCommand.Flags().StringVar(&serverCommandContext.metricsAddr, "metrics-bind-address", ":8080",
		"The address the metric endpoint binds to.")
	serverCommand.Flags().StringVar(&serverCommandContext.probeAddr, "health-probe-bind-address", ":8081",
		"The address the probe endpoint binds to.")
	serverCommand.Flags().BoolVar(&serverCommandContext.enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
}

func server() error {
	ctrl.SetLogger(packageLog)
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     serverCommandContext.metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: serverCommandContext.probeAddr,
		LeaderElection:         serverCommandContext.enableLeaderElection,
		LeaderElectionID:       "6ef7a950.eks.amazonaws.com",
	})
	if err != nil {
		return fmt.Errorf("unable to start manager: %v", err)
	}

	if err = controllers.RegisterPackageBundleReconciler(mgr); err != nil {
		return fmt.Errorf("unable to register package bundle controller: %v", err)
	}
	if err = controllers.RegisterPackageBundleControllerReconciler(mgr); err != nil {
		return fmt.Errorf("unable to register package bundle controller controller: %v", err)
	}
	if err = controllers.RegisterPackageReconciler(mgr); err != nil {
		return fmt.Errorf("unable to register package controller: %v", err)
	}

	if os.Getenv("ENABLE_WEBHOOKS") == "true" {
		if err := webhook.InitPackageBundleValidator(mgr); err != nil {
			return fmt.Errorf("unable to create package bundle webhook: %v", err)
		}
		if err := webhook.InitPackageBundleControllerValidator(mgr); err != nil {
			return fmt.Errorf("unable to create package bundle controller webhook: %v", err)
		}
		if err = webhook.InitPackageValidator(mgr); err != nil {
			return fmt.Errorf("unable to create package webhook: %v", err)
		}
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check")
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up ready check")
	}

	packageLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		return fmt.Errorf("problem running manager: %v", err)
	}

	return nil
}

func runServer(_ *cobra.Command, _ []string) {
	err := server()
	if err != nil {
		packageLog.Error(err, "server")
	}
}

var serverCommand = &cobra.Command{
	Use:   "server",
	Short: "Run package controller server",
	Long:  "Run package controller server",
	Run:   runServer,
}
