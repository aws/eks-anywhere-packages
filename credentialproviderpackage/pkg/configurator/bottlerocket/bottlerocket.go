package bottlerocket

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/aws/eks-anywhere-packages/credentialproviderpackage/pkg/configurator"
	"github.com/aws/eks-anywhere-packages/credentialproviderpackage/pkg/constants"
)

type bottleRocket struct {
	client  http.Client
	baseURL string
	config  constants.CredentialProviderConfigOptions
}

type awsCred struct {
	Aws aws `json:"aws"`
}
type aws struct {
	Config  string `json:"config"`
	Profile string `json:"profile"`
	Region  string `json:"region"`
}

type brKubernetes struct {
	Kubernetes kubernetes `json:"kubernetes"`
}
type ecrCredentialProvider struct {
	CacheDuration string   `json:"cache-duration"`
	Enabled       bool     `json:"enabled"`
	ImagePatterns []string `json:"image-patterns"`
}
type credentialProviders struct {
	EcrCredentialProvider ecrCredentialProvider `json:"ecr-credential-provider"`
}
type kubernetes struct {
	CredentialProviders credentialProviders `json:"credential-providers"`
}

type brVersion struct {
	Os struct {
		Arch       string `json:"arch"`
		BuildID    string `json:"build_id"`
		PrettyName string `json:"pretty_name"`
		VariantID  string `json:"variant_id"`
		VersionID  string `json:"version_id"`
	} `json:"os"`
}

var _ configurator.Configurator = (*bottleRocket)(nil)

func NewBottleRocketConfigurator(socketPath string) (*bottleRocket, error) {
	socket, err := os.Stat(socketPath)
	if err != nil {
		return nil, err
	}
	if socket.Mode().Type() != fs.ModeSocket {
		return nil, fmt.Errorf("Unexpected type %s expected socket\n", socket.Mode().Type())
	}

	br := &bottleRocket{
		baseURL: "http://localhost/",
		client: http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			},
		},
	}

	valid, err := br.isSupportedBRVersion()
	if err != nil {
		return nil, fmt.Errorf("error retrieving BR version %v", err)
	}
	if !valid {
		return nil, fmt.Errorf("unsupported BR version %v", err)
	}
	return br, nil
}

func (b *bottleRocket) Initialize(config constants.CredentialProviderConfigOptions) {
	b.config = config
}

func (b *bottleRocket) UpdateAWSCredentials(path string, profile string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	content := base64.StdEncoding.EncodeToString(data)
	payload, err := createCredentialsPayload(content, profile)
	if err != nil {
		return err
	}
	err = b.sendSettingsSetRequest(payload)
	if err != nil {
		return err
	}

	err = b.CommitChanges()
	if err != nil {
		return err
	}

	return err
}

func (b *bottleRocket) UpdateCredentialProvider(_ string) error {
	payload, err := createCredentialProviderPayload(b.config)
	if err != nil {
		return err
	}
	err = b.sendSettingsSetRequest(payload)
	if err != nil {
		return err
	}

	return err
}

func (b *bottleRocket) CommitChanges() error {
	// For Bottlerocket this step is committing all changes at once
	commitPath := b.baseURL + "tx/commit_and_apply"
	resp, err := b.client.Post(commitPath, "application/json", bytes.NewBuffer(make([]byte, 0)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to commit changes: %s", resp.Status)
	}
	return nil
}

func (b *bottleRocket) sendSettingsSetRequest(payload []byte) error {
	settingsPath := b.baseURL + "settings"
	req, err := http.NewRequest(http.MethodPatch, settingsPath, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	respPatch, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer respPatch.Body.Close()

	if respPatch.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed patch request: %s", respPatch.Status)
	}

	return nil

}

func createCredentialsPayload(content string, profile string) ([]byte, error) {
	aws := aws{
		Config:  content,
		Profile: profile,
	}

	creds := awsCred{Aws: aws}

	payload, err := json.Marshal(creds)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func createCredentialProviderPayload(config constants.CredentialProviderConfigOptions) ([]byte, error) {
	providerConfig := brKubernetes{
		Kubernetes: kubernetes{
			credentialProviders{
				ecrCredentialProvider{
					Enabled:       true,
					ImagePatterns: config.ImagePatterns,
					CacheDuration: config.DefaultCacheDuration,
				},
			},
		},
	}

	payload, err := json.Marshal(providerConfig)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func (b *bottleRocket) isSupportedBRVersion() (bool, error) {
	allowedVersions := "v1.11.0"
	req, err := http.NewRequest(http.MethodGet, b.baseURL, nil)
	if err != nil {
		return false, err
	}
	q := req.URL.Query()
	q.Add("prefix", "os")
	req.URL.RawQuery = q.Encode()

	respGet, err := b.client.Do(req)
	if err != nil {
		return false, err
	}

	defer respGet.Body.Close()

	if respGet.StatusCode != http.StatusOK {
		return false, fmt.Errorf("failed GET request: %s", respGet.Status)
	}

	valueBody, err := ioutil.ReadAll(respGet.Body)
	if err != nil {
		return false, err
	}

	osVersion := brVersion{}
	err = json.Unmarshal(valueBody, &osVersion)
	if err != nil {
		return false, err
	}

	// Check if BR k8s version is 1.25 or greater. If so we will update allowedVersions to be >1.13.0
	if osVersion.Os.VariantID != "" {
		variants := strings.Split(osVersion.Os.VariantID, "-")
		variantSemVar := "v" + variants[len(variants)-1]

		if semver.Compare(variantSemVar, "v1.25") >= 0 {
			allowedVersions = "v1.13.0"
		}
	}

	ver := "v" + osVersion.Os.VersionID
	valid := semver.Compare(ver, allowedVersions)
	return valid > 0, nil
}
