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

package v1alpha1_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"

	//+kubebuilder:scaffold:imports
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/bundle"
	"github.com/aws/eks-anywhere-packages/pkg/signature"
	"github.com/aws/eks-anywhere-packages/pkg/testutil"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	//	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestAPIs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping API test in short mode")
	}
	// These tests require envtest. The project Makefile, if used, should set
	// this env var appropriately. If it's not set, we're probably being run via
	// go test directly, and so we skip. This allows for faster test cycles in
	// dev.
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		t.Skip("skipping API test because envtest environment found")
	}
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Webhook Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = Context("Releases signatures are validated against the correct public key", func() {
	ctx := context.Background()
	When("a release is applied", func() {
		It("validates the signature against the default key (production)", func() {
			bundle, _, err := testutil.GivenPackageBundle("../testdata/bundle_one.yaml")
			Expect(err).ShouldNot(HaveOccurred())
			err = k8sClient.Create(ctx, bundle)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("The signature is invalid"))
			Expect(err.Error()).Should(ContainSubstring(signature.EksaDomain.Pubkey))
		})

		It("validates the signature against the overriden key (using env var)", func() {
			//Test public key
			err := os.Setenv(PublicKeyEnvVar, "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEvME/v61IfA4ulmgdF10Ae/WCRqtXvrUtF+0nu0dbdP36u3He4GRepYdQGCmbPe0463yAABZs01/Vv/v52ktlmg==")
			Expect(err).ShouldNot(HaveOccurred())
			bundle, _, err := testutil.GivenPackageBundle("../testdata/bundle_one.yaml")
			Expect(err).ShouldNot(HaveOccurred())
			err = k8sClient.Create(ctx, bundle)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("validates the signature against the default key if the environment variable exists but is empty", func() {
			err := os.Setenv(PublicKeyEnvVar, "")
			Expect(err).ShouldNot(HaveOccurred())
			bundle, _, err := testutil.GivenPackageBundle("../testdata/bundle_one.yaml")
			Expect(err).ShouldNot(HaveOccurred())
			err = k8sClient.Create(ctx, bundle)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("The signature is invalid"))
			Expect(err.Error()).Should(ContainSubstring(signature.EksaDomain.Pubkey))
		})
	})
})
var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: false,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "config", "webhook")},
		},
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	scheme := runtime.NewScheme()
	err = AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = corev1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = admissionv1beta1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// start webhook server using Manager
	webhookInstallOptions := &testEnv.WebhookInstallOptions
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme,
		Host:               webhookInstallOptions.LocalServingHost,
		Port:               webhookInstallOptions.LocalServingPort,
		CertDir:            webhookInstallOptions.LocalServingCertDir,
		LeaderElection:     false,
		MetricsBindAddress: "0",
	})
	Expect(err).NotTo(HaveOccurred())

	err = (&PackageBundle{}).SetupWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:webhook

	go func() {
		defer GinkgoRecover()
		err = mgr.Start(ctx)
		Expect(err).NotTo(HaveOccurred())
	}()

	// wait for the webhook server to get ready
	dialer := &net.Dialer{Timeout: time.Second}
	addrPort := fmt.Sprintf("%s:%d", webhookInstallOptions.LocalServingHost, webhookInstallOptions.LocalServingPort)
	Eventually(func() error {
		conn, err := tls.DialWithDialer(dialer, "tcp", addrPort, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}).Should(Succeed())

	ns := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: bundle.ActiveBundleNamespace},
	}
	Expect(k8sClient.Create(ctx, &ns)).Should(Succeed())
}, 60)

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
