package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestKubernetes137BundleInputs(t *testing.T) {
	for _, stage := range []string{"dev", "staging", "prod"} {
		t.Run(stage, func(t *testing.T) {
			input, err := ValidateInputConfig(filepath.Join("data", "bundles_"+stage, "1-37.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			if input.KubernetesVersion != "1.37" || input.Name != "v1-37-1001" {
				t.Fatal("bundle input does not identify Kubernetes 1.37")
			}
			autoscalerFound := false
			for _, org := range input.Packages {
				for _, project := range org.Projects {
					if project.Name == "cluster-autoscaler" {
						autoscalerFound = true
						if !strings.HasPrefix(project.Versions[0].Name, "9.55.0-1.37-") {
							t.Fatal("autoscaler chart does not match Kubernetes 1.37")
						}
					}
					if stage != "dev" {
						for _, version := range project.Versions {
							if strings.HasSuffix(version.Name, "-latest") {
								t.Fatalf("release input has an unpinned chart: %s", project.Name)
							}
						}
					}
				}
			}
			if !autoscalerFound {
				t.Fatal("autoscaler chart missing")
			}
		})
	}
}

func TestBundleVersionsDiscoverNewInputs(t *testing.T) {
	root := t.TempDir()
	inputs := filepath.Join(root, "generatebundlefile", "data", "bundles_dev")
	if err := os.MkdirAll(inputs, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"1-36.yaml", "1-37.yaml", "1-38.yaml"} {
		if err := os.WriteFile(filepath.Join(inputs, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	output, err := bundleVersions(root, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "1-36\n1-37\n1-38\n" {
		t.Fatalf("unexpected discovered versions: %q", output)
	}
}

func TestBundleVersionsMissingInputs(t *testing.T) {
	if _, err := bundleVersions(t.TempDir(), "dev"); err == nil {
		t.Fatal("missing bundle inputs must fail instead of publishing nothing")
	}
}

func TestBundleVersionsInvalidInput(t *testing.T) {
	root := t.TempDir()
	inputs := filepath.Join(root, "generatebundlefile", "data", "bundles_dev")
	if err := os.MkdirAll(inputs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputs, "invalid.yaml"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := bundleVersions(root, "dev"); err == nil {
		t.Fatal("invalid input filename must fail")
	}
}

func bundleVersions(root, stage string) ([]byte, error) {
	return exec.Command("bash", "-c", `source "$1"; BASE_DIRECTORY="$2"; bundle_versions "$3"`,
		"bash", filepath.Join("hack", "common.sh"), root, stage).CombinedOutput()
}
