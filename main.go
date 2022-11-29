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

package main

import (
	"context"
	"flag"
	"fmt"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/controllers"
	//"github.com/aws/eks-anywhere-packages/pkg/packages"
	"github.com/aws/eks-anywhere-packages/pkg/webhook"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	fmt.Println("ASDFASDFASDFASDFASDF")
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var migrate bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&migrate, "migrate", false, "Migrate from older release")
	opts := zap.Options{
		Development: true,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if migrate {
		err := runMigration()
		if err != nil {
			setupLog.Error(err, "migration failed")
			os.Exit(-1)
		}
		os.Exit(0)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "6ef7a950.eks.amazonaws.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = controllers.RegisterPackageBundleReconciler(mgr); err != nil {
		setupLog.Error(err, "unable to register controller", "controller", "PackageBundle")
		os.Exit(1)
	}
	if err = controllers.RegisterPackageBundleControllerReconciler(mgr); err != nil {
		setupLog.Error(err, "unable to register controller", "controller", "PackageBundleController")
		os.Exit(1)
	}
	if err = controllers.RegisterPackageReconciler(mgr); err != nil {
		setupLog.Error(err, "unable to register controller", "controller", "Package")
		os.Exit(1)
	}

	if os.Getenv("ENABLE_WEBHOOKS") == "true" {
		if err := webhook.InitPackageBundleValidator(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "PackageBundle")
			os.Exit(1)
		}
		if err := webhook.InitPackageBundleControllerValidator(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "PackageBundleController")
			os.Exit(1)
		}
		if err = webhook.InitPackageValidator(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Package")
			os.Exit(1)
		}
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func getKubeConfig() string {
	kubeconfig := os.Getenv("KUBECONFIG")
	if len(kubeconfig) == 0 {
		kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}
	return kubeconfig
}

func runMigration() error {
	fmt.Println("START")
	ctx := context.Background()

	clusterName := os.Getenv("CLUSTER_NAME")
	if len(clusterName) == 0 {
		return fmt.Errorf("CLUSTER_NAME environment variable is not set or exported")
	}
	clusterNamespace := "eksa-packages-" + clusterName

	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", getKubeConfig())
		if err != nil {
			return fmt.Errorf("building Kubernetes configuration: %v", err)
		}
	}

	err = v1alpha1.AddToScheme(scheme)
	if err != nil {
		return fmt.Errorf("add schema: %v", err)
	}

	clientOptions := client.Options{
		Scheme: scheme,
	}
	rtClient, err := client.New(config, clientOptions)
	if err != nil {
		return fmt.Errorf("creating Kubernetes runtime client: %v", err)
	}

	packageList := v1alpha1.PackageList{}
	err = rtClient.List(ctx, &packageList, &client.ListOptions{Namespace: "eksa-packages"})
	if err != nil {
		return fmt.Errorf("reading: %v", err)
	}

	fmt.Println("list------")
	for _, pkg := range packageList.Items {
		fmt.Println("package", "name", pkg.Name)
		newPackage := v1alpha1.Package{}
		err = rtClient.Get(ctx, client.ObjectKey{Namespace: clusterNamespace, Name: pkg.Name}, &newPackage)
		if err != nil {
			pkg.Namespace = clusterNamespace
			pkg.ResourceVersion = ""
			pkg.UID = ""
			err = rtClient.Create(ctx, &pkg, &client.CreateOptions{})
			if err != nil {
				return fmt.Errorf("update error: %v", err)
			}
		} else {
			pkg.Namespace = clusterNamespace
			pkg.ResourceVersion = newPackage.ResourceVersion
			pkg.UID = newPackage.UID
			fmt.Println("newPackage.Name: " + newPackage.Name)
			fmt.Println("newPackage.Namespace: " + newPackage.Namespace)
			fmt.Println("newPackage.UID: " + newPackage.UID)
			err = rtClient.Update(ctx, &pkg, &client.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("update error: %v", err)
			}
		}
	}
	fmt.Println("STOP")
	return nil
}
