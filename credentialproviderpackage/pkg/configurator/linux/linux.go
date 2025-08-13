package linux

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	ps "github.com/mitchellh/go-ps"
	"golang.org/x/mod/semver"

	"github.com/aws/eks-anywhere-packages/credentialproviderpackage/pkg/configurator"
	"github.com/aws/eks-anywhere-packages/credentialproviderpackage/pkg/constants"
	"github.com/aws/eks-anywhere-packages/credentialproviderpackage/pkg/log"
	"github.com/aws/eks-anywhere-packages/credentialproviderpackage/pkg/templater"
)

//go:embed templates/credential-provider-config.yaml
var credProviderTemplate string

const (
	binPath               = "/eksa-binaries/"
	basePath              = "/eksa-packages/"
	credOutFile           = "aws-creds"
	mountedExtraArgs      = "/node-files/kubelet-extra-args"
	ubuntuLegacyExtraArgs = "/node-files/ubuntu-legacy-kubelet-extra-args"
	credProviderFile      = "credential-provider-config.yaml"

	// Binaries
	ecrCredProviderBinary = "ecr-credential-provider"
	iamRolesSigningBinary = "aws_signing_helper"
)

type linuxOS struct {
	profile             string
	extraArgsPath       string
	legacyExtraArgsPath string
	basePath            string
	config              constants.CredentialProviderConfigOptions
}

var _ configurator.Configurator = (*linuxOS)(nil)

func NewLinuxConfigurator() *linuxOS {
	return &linuxOS{
		profile:             "",
		extraArgsPath:       mountedExtraArgs,
		legacyExtraArgsPath: ubuntuLegacyExtraArgs,
		basePath:            basePath,
	}
}

func (c *linuxOS) Initialize(config constants.CredentialProviderConfigOptions) {
	c.config = config
}

func (c *linuxOS) UpdateAWSCredentials(sourcePath, profile string) error {
	c.profile = profile
	dstPath := c.basePath + credOutFile

	err := copyWithPermissons(sourcePath, dstPath, 0o600)
	return err
}

func (c *linuxOS) updateConfigFile(configPath string) error {
	file, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(file), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, "KUBELET_EXTRA_ARGS") {
			found = true
			args := c.updateKubeletArguments(line)

			if args != "" {
				lines[i] = line + args + "\n"
			}
		}
	}
	if !found {
		line := "KUBELET_EXTRA_ARGS="
		args := c.updateKubeletArguments(line)
		if args != "" {
			line = line + args
		}
		lines = append(lines, line)
	}

	out := strings.Join(lines, "\n")
	err = os.WriteFile(configPath, []byte(out), 0o644)
	return err
}

func (c *linuxOS) UpdateCredentialProvider(_ string) error {
	// Adding to KUBELET_EXTRA_ARGS in place
	if err := c.updateConfigFile(mountedExtraArgs); err != nil {
		return fmt.Errorf("failed to update kubelet args: %v", err)
	}

	// Adding KUBELET_EXTRA_ARGS to legacy path for ubuntu
	if _, err := os.Stat(ubuntuLegacyExtraArgs); err == nil {
		if err := c.updateConfigFile(ubuntuLegacyExtraArgs); err != nil {
			return fmt.Errorf("failed to update legacy kubelet args for ubuntu: %v", err)
		}
	}

	return nil
}

func (c *linuxOS) CommitChanges() error {
	process, err := findKubeletProcess()
	if err != nil {
		return err
	}
	err = killProcess(process)
	return err
}

func killProcess(process ps.Process) error {
	err := syscall.Kill(process.Pid(), syscall.SIGHUP)
	return err
}

func findKubeletProcess() (ps.Process, error) {
	processList, err := ps.Processes()
	if err != nil {
		return nil, err
	}
	for x := range processList {
		process := processList[x]
		if process.Executable() == "kubelet" {
			return process, nil
		}
	}
	return nil, fmt.Errorf("cannot find Kubelet Process")
}

func getApiVersion() string {
	k8sVersion := os.Getenv("K8S_VERSION")
	apiVersion := "v1"
	if semver.Compare(k8sVersion, "v1.25") <= 0 {
		apiVersion = "v1alpha1"
	}
	if k8sVersion == "" {
		apiVersion = "v1"
	}
	return apiVersion
}

func copyWithPermissons(srcpath, dstpath string, permission os.FileMode) (err error) {
	r, err := os.Open(srcpath)
	if err != nil {
		return err
	}
	defer r.Close() // ok to ignore error: file was opened read-only.

	w, err := os.Create(dstpath)
	if err != nil {
		return err
	}

	defer func() {
		c := w.Close()
		// Report the error from Close, if any.
		// But do so only if there isn't already
		// an outgoing error.
		if c != nil && err == nil {
			err = c
		}
	}()

	_, err = io.Copy(w, r)
	if err != nil {
		return err
	}
	err = os.Chmod(dstpath, permission)
	return err
}

func copyBinaries() (string, error) {
	srcPath := binPath + getApiVersion() + "/" + ecrCredProviderBinary
	dstPath := basePath + ecrCredProviderBinary
	err := copyWithPermissons(srcPath, dstPath, 0o700)
	if err != nil {
		return "", err
	}

	err = os.Chmod(dstPath, 0o700)
	if err != nil {
		return "", err
	}

	srcPath = binPath + iamRolesSigningBinary
	dstPath = basePath + iamRolesSigningBinary
	err = copyWithPermissons(srcPath, dstPath, 0o700)
	if err != nil {
		return "", err
	}

	err = os.Chmod(dstPath, 0o700)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(" --image-credential-provider-bin-dir=%s", basePath), nil
}

func (c *linuxOS) createConfig() (string, error) {
	values := map[string]interface{}{
		"profile":       c.profile,
		"config":        basePath + credOutFile,
		"home":          basePath,
		"apiVersion":    getApiVersion(),
		"imagePattern":  c.config.ImagePatterns,
		"cacheDuration": c.config.DefaultCacheDuration,
		"proxy":         configurator.GetProxyEnvironment(),
	}

	dstPath := c.basePath + credProviderFile

	bytes, err := templater.Execute(credProviderTemplate, values)
	if err != nil {
		return "", err
	}
	err = os.WriteFile(dstPath, bytes, 0o600)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(" --image-credential-provider-config=%s", dstPath), nil
}

func (c *linuxOS) updateKubeletArguments(line string) string {
	args := ""

	val, err := c.createConfig()
	if err != nil {
		log.ErrorLogger.Printf("Error creating configuration %v", err)
	}
	// We want to upgrade the eksa owned configuration/binaries everytime however,
	// we don't want to update what configuration is being pointed to in cases of a custom config
	if !strings.Contains(line, "image-credential-provider-config") {
		args += val
	}

	val, err = copyBinaries()
	if err != nil {
		log.ErrorLogger.Printf("Error coping binaries %v\n", err)
	}
	if !strings.Contains(line, "image-credential-provider-bin-dir") {
		args += val
	}
	return args
}
