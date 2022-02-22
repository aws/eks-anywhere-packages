package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/octago/sflags/gen/gpflag"
	"k8s.io/klog"
	"sigs.k8s.io/kubetest2/pkg/exec"
)

func main() {
	t := NewTester()
	err := t.Execute()
	if err != nil {
		klog.Fatalf("failed to run eksa tester: %s", err)
	}
}

func NewTester() *Tester {
	return &Tester{}
}

type Tester struct {
	SourcePath string `desc:"Path to the modelrocket-add-ons source."`

	kubeconfigPath string
}

func (t *Tester) Execute() error {
	fs, err := gpflag.Parse(t)
	if err != nil {
		return fmt.Errorf("failed to initialize tester: %s", err)
	}

	help := fs.BoolP("help", "h", false, "")
	err = fs.Parse(os.Args)
	if err != nil {
		return fmt.Errorf("failed to parse flags: %s", err)
	}
	if *help {
		fs.PrintDefaults()
		return nil
	}

	err = t.setup()
	if err != nil {
		return err
	}

	err = t.runMakefileTests()
	if err != nil {
		return err
	}

	return nil
}

func (t *Tester) setup() (err error) {
	if t.SourcePath == "" {
		t.SourcePath, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("error getting working directory: %s", err)
		}
	}

	err = t.findKubeconfig()
	if err != nil {
		return err
	}

	return nil
}

func (t *Tester) findKubeconfig() (err error) {
	config := os.Getenv("KUBECONFIG")
	if config != "" {
		if !filepath.IsAbs(config) {
			newConfig, err := filepath.Abs(config)
			if err != nil {
				return fmt.Errorf("determining absolute kubeconfig path: %s", err)
			}
			config = newConfig
		}
		t.kubeconfigPath = config
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("finding user's home dir for KUBECONFIG: %s", err)
		}
		t.kubeconfigPath = filepath.Join(home, ".kube", "config")
	}

	return nil
}

func (t *Tester) runMakefileTests() (err error) {
	if t.SourcePath != "" {
		err = os.Chdir(t.SourcePath)
		if err != nil {
			return fmt.Errorf("changing to source directory: %s", err)
		}
	}

	cmd := exec.Command("make", "install", "test-e2e-smoke")
	exec.InheritOutput(cmd)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("running makefile e2e tests: %s", err)
	}
	klog.Infof("ran makefile e2e tests")

	return nil
}
