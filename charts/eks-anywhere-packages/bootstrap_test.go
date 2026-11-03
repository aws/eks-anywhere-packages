package chart_test

import (
	"strings"
	"testing"

	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	"sigs.k8s.io/yaml"

	packagesv1 "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

type chartClientProvider struct {
	client dynamic.Interface
}

func (p chartClientProvider) GetClientFor(apiVersion, kind string) (dynamic.NamespaceableResourceInterface, bool, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, false, err
	}
	resources := map[string]string{"Node": "nodes", "Secret": "secrets", "CronJob": "cronjobs",
		"Package": "packages", "PackageBundleController": "packagebundlecontrollers"}
	return p.client.Resource(gv.WithResource(resources[kind])), kind != "Node", nil
}

func renderBootstrap(t *testing.T, existing ...runtime.Object) map[string]string {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := packagesv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	existing = append(existing, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node"},
		Status: corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{OSImage: "Ubuntu 22.04.5 LTS"}}})
	client := fake.NewSimpleDynamicClient(scheme, existing...)
	chrt, err := loader.Load(".")
	if err != nil {
		t.Fatal(err)
	}
	values, err := chartutil.ToRenderValues(chrt, map[string]interface{}{
		"clusterName": "test-cluster", "cronjob": map[string]interface{}{"suspend": true},
		"awsSecret": map[string]interface{}{"secret": "dummy"},
	}, chartutil.ReleaseOptions{Name: "eks-anywhere-packages", Namespace: "eksa-packages", IsInstall: true}, chartutil.DefaultCapabilities)
	if err != nil {
		t.Fatal(err)
	}
	out, err := engine.RenderWithClientProvider(chrt, values, chartClientProvider{client: client})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestBootstrapResourcesRunAfterControllerReadiness(t *testing.T) {
	out := renderBootstrap(t)
	for _, name := range []string{"packagebundlecontroller.yaml", "package.yaml", "credentialpackage.yaml"} {
		content := out["eks-anywhere-packages/templates/"+name]
		var resource struct {
			Kind     string `json:"kind"`
			Metadata struct {
				Annotations map[string]string `json:"annotations"`
			} `json:"metadata"`
		}
		if err := yaml.Unmarshal([]byte(content), &resource); err != nil {
			t.Fatal(err)
		}
		if resource.Kind == "" || resource.Metadata.Annotations["helm.sh/hook"] != "post-install,post-upgrade" {
			t.Fatalf("%s is admitted before the controller is ready: %s", name, content)
		}
		if resource.Metadata.Annotations["helm.sh/resource-policy"] != "keep" {
			t.Fatalf("%s is not retained", name)
		}
	}
	if !strings.Contains(out["eks-anywhere-packages/templates/webhook.yaml"], "failurePolicy: Fail") {
		t.Fatal("admission failure policy must stay Fail")
	}
}

func TestBootstrapResourcesPreserveExistingState(t *testing.T) {
	out := renderBootstrap(t,
		&packagesv1.PackageBundleController{ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "eksa-packages"}},
		&packagesv1.Package{ObjectMeta: metav1.ObjectMeta{Name: "eks-anywhere-packages", Namespace: "eksa-packages-test-cluster"}},
		&packagesv1.Package{ObjectMeta: metav1.ObjectMeta{Name: "ecr-credential-provider-package", Namespace: "eksa-packages-test-cluster"}},
	)
	for _, name := range []string{"packagebundlecontroller.yaml", "package.yaml", "credentialpackage.yaml"} {
		if strings.TrimSpace(out["eks-anywhere-packages/templates/"+name]) != "" {
			t.Fatalf("%s would replace an existing bootstrap object", name)
		}
	}
}
