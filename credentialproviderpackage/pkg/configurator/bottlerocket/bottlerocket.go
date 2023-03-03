package bottlerocket

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"

	"credential-provider/pkg/configurator"
	"credential-provider/pkg/constants"
)

type bottleRocket struct {
	client  http.Client
	baseURL string
	config  constants.CredentialProviderConfigOptions
}

type awsCred struct {
	Aws Aws `json:"aws"`
}
type Aws struct {
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

var _ configurator.Configurator = (*bottleRocket)(nil)

func NewBottleRocketConfigurator(socketPath string) *bottleRocket {
	return &bottleRocket{
		client: http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			},
		},
	}
}

func (b *bottleRocket) Initialize(config constants.CredentialProviderConfigOptions) {
	b.baseURL = "http://localhost/"
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
	aws := Aws{
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
