package linux

import (
	_ "embed"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"syscall"

	ps "github.com/mitchellh/go-ps"

	"credential-provider/pkg/configurator"
	"credential-provider/pkg/constants"
	"credential-provider/pkg/log"
	"credential-provider/pkg/templater"
)

//go:embed templates/credential-provider-config.yaml
var credProviderTemplate string

type linuxOS struct {
	profile       string
	extraArgsPath string
	basePath      string
	config        constants.CredentialProviderConfigOptions
}

var _ configurator.Configurator = (*linuxOS)(nil)

func NewLinuxConfigurator() *linuxOS {
	return &linuxOS{
		profile:       "",
		extraArgsPath: constants.MountedExtraArgs,
		basePath:      constants.BasePath,
	}
}

func (c *linuxOS) Initialize(config constants.CredentialProviderConfigOptions) {
	c.config = config
}

func (c *linuxOS) UpdateAWSCredentials(sourcePath string, profile string) error {
	c.profile = profile
	dstPath := c.basePath + constants.CredOutFile

	err := copyWithPermissons(sourcePath, dstPath, 0600)
	return err
}

func (c *linuxOS) UpdateCredentialProvider(_ string) error {
	// Adding to KUBELET_EXTRA_ARGS in place
	file, err := ioutil.ReadFile(c.extraArgsPath)
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
	err = ioutil.WriteFile(c.extraArgsPath, []byte(out), 0644)
	if err != nil {
		return err
	}
	return nil
}

func (c *linuxOS) CommitChanges() error {
	process, err := findKubeletProcess()
	if err != nil {
		return err
	}
	err = killProcess(process)
	if err != nil {
		return err
	}
	return nil
}

func killProcess(process ps.Process) error {
	err := syscall.Kill(process.Pid(), syscall.SIGHUP)
	if err != nil {
		return err
	}
	return nil
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
	srcPath := constants.BinPath + constants.ECRCredProviderBinary
	dstPath := constants.BasePath + constants.ECRCredProviderBinary
	err := copyWithPermissons(srcPath, dstPath, 0744)
	if err != nil {
		return "", err
	}

	err = os.Chmod(dstPath, 0744)
	if err != nil {
		return "", err
	}

	srcPath = constants.BinPath + constants.IAMRolesSigningBinary
	dstPath = constants.BasePath + constants.IAMRolesSigningBinary
	err = copyWithPermissons(srcPath, dstPath, 0744)
	if err != nil {
		return "", err
	}

	err = os.Chmod(dstPath, 0744)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(" --image-credential-provider-bin-dir=%s", constants.BasePath), nil
}

func (c *linuxOS) createConfig() (string, error) {
	values := map[string]interface{}{
		"profile":       c.profile,
		"config":        constants.BasePath + constants.CredOutFile,
		"home":          constants.BasePath,
		"imagePattern":  c.config.ImagePatterns,
		"cacheDuration": c.config.DefaultCacheDuration,
	}

	dstPath := c.basePath + constants.CredProviderFile

	bytes, err := templater.Execute(credProviderTemplate, values)
	if err != nil {
		return "", nil
	}
	err = ioutil.WriteFile(dstPath, bytes, 0644)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(" --image-credential-provider-config=%s", dstPath), nil
}

func (c *linuxOS) updateKubeletArguments(line string) string {
	args := ""
	if !strings.Contains(line, "KubeletCredentialProviders") {
		args += " --feature-gates=KubeletCredentialProviders=true"
	}

	if !strings.Contains(line, "image-credential-provider-config") {
		val, err := c.createConfig()
		if err != nil {
			log.ErrorLogger.Printf("Error creating configuration %v", err)
		}
		args += val

		val, err = copyBinaries()
		if err != nil {
			log.ErrorLogger.Printf("Error coping binaries %v\n", err)
		}
		if !strings.Contains(line, "image-credential-provider-bin-dir") {
			args += val
		}
	}
	return args
}
