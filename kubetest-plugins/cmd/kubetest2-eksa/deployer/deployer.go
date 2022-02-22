package deployer

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/octago/sflags/gen/gpflag"
	"github.com/spf13/pflag"
	"k8s.io/klog"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kubetest2/pkg/metadata"
	"sigs.k8s.io/kubetest2/pkg/process"
	"sigs.k8s.io/kubetest2/pkg/types"
)

const Name = "eksa"

var GitTag string = "<unknown>" // set via Makefile

func New(opts types.Options) (types.Deployer, *pflag.FlagSet) {
	d := &deployer{
		commonOptions: opts,
	}
	return d, bindFlags(d)
}

var _ types.NewDeployer = New

type deployer struct {
	commonOptions types.Options

	ClusterName       string `flag:"cluster-name" desc:"the eksa cluster name"`
	ClusterConfigPath string `flag:"cluster-config" desc:"the cluster config file path"`
	KubeconfigPath    string `flag:"kubeconfig" desc:"--kubeconfig flag for eksctl anywhere create cluster"`
	Provider          string `flag:"provider" desc:"--provider flag for eksctl anywhere generate clusterconfig"`
	ForceCleanup      bool   `flag:"force-cleanup" desc:"Force deletion of previously created bootstrap cluster"`
	Verbosity         *int   `flag:"verbosity" desc:"Set the log level verbosity"`
}

func (d *deployer) Up() error {
	filename, err := d.generate()
	if err != nil {
		return err
	}

	args := []string{
		"anywhere",
		"create",
		"cluster",
		fmt.Sprintf("--filename=%s", filename),
	}

	if d.Verbosity != nil {
		args = append(args, fmt.Sprintf("--verbosity=%d", d.Verbosity))
	}
	if d.ForceCleanup {
		args = append(args, "--force-cleanup")
	}

	klog.V(0).Infof("Up(): creating eks anywhere cluster...\n")
	return process.ExecJUnit("eksctl", args, os.Environ())
}

func (d *deployer) generate() (string, error) {
	if d.Provider == "" {
		d.Provider = "docker"
	}
	name := "eksa-e2e-tests"
	if d.ClusterName != "" {
		name = d.ClusterName
	}
	d.ClusterConfigPath = filepath.Join(d.commonOptions.RunDir(), name+".yaml")
	f, err := os.OpenFile(d.ClusterConfigPath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return "", fmt.Errorf("error creating file for eks-a cluster config: %s", err)
	}
	defer f.Close()

	origStdout := os.Stdout
	os.Stdout = f
	defer func() { os.Stdout = origStdout }()

	args := []string{
		"anywhere",
		"generate",
		"clusterconfig",
		name,
		"--provider",
		d.Provider,
	}

	err = os.Chdir(d.commonOptions.RunDir())
	if err != nil {
		return "", fmt.Errorf("error changing to RunDir: %s", err)
	}

	klog.V(0).Infof("Up(): generating eks anywhere cluster config...\n")
	err = process.ExecJUnit("eksctl", args, os.Environ())
	if err != nil {
		return "", fmt.Errorf("error generating eks-a cluster config: %s", err)
	}

	return d.ClusterConfigPath, nil
}

// eksctl anywhere delete cluster dev -w ~/run/eksa/dev/dev-eks-a-cluster.kubeconfig
func (d *deployer) Down() error {
	args := []string{
		"anywhere",
		"delete",
		"cluster",
		d.ClusterName,
	}

	if d.ClusterConfigPath != "" {
		args = append(args, "--filename", d.ClusterConfigPath)
	}
	if d.KubeconfigPath != "" {
		args = append(args, "--w-config", d.KubeconfigPath)
	}

	klog.V(0).Infof("Down(): deleting eks anywhere cluster...\n")
	// we want to see the output so use process.ExecJUnit
	return process.ExecJUnit("eksctl", args, os.Environ())
}

func (d *deployer) IsUp() (up bool, err error) {
	// naively assume that if the api server reports nodes, the cluster is up
	lines, err := exec.CombinedOutputLines(
		exec.Command("kubectl", "get", "nodes", "-o=name"),
	)
	if err != nil {
		return false, metadata.NewJUnitError(err, strings.Join(lines, "\n"))
	}
	return len(lines) > 0, nil
}

func (d *deployer) DumpClusterLogs() error {
	return nil
}

func (d *deployer) Build() error {
	return nil
}

func (d *deployer) Kubeconfig() (string, error) {
	return filepath.Join(d.commonOptions.RunDir(), d.ClusterName,
		fmt.Sprintf("%s-eks-a-cluster.kubeconfig", d.ClusterName)), nil
}

func (d *deployer) Version() string {
	return GitTag
}

var _ types.DeployerWithKubeconfig = &deployer{}

// helper used to create & bind a flagset to the deployer
func bindFlags(d *deployer) *pflag.FlagSet {
	flags, err := gpflag.Parse(d)
	if err != nil {
		klog.Fatalf("unable to generate flags from deployer")
		return nil
	}

	klog.InitFlags(nil)
	flags.AddGoFlagSet(flag.CommandLine)

	return flags
}
