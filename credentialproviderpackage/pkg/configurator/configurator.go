package configurator

import "credential-provider/pkg/constants"

type Configurator interface {
	// Initialize Handles node specific configuration depending on OS
	Initialize(filepath string, config constants.CredentialProviderConfigOptions)

	// UpdateAWSCredentials Handles AWS Credential Setup
	UpdateAWSCredentials(sourcePath string, profile string) error

	// UpdateCredentialProvider Handles Credential Provider Setup
	UpdateCredentialProvider(profile string) error

	// CommitChanges Applies changes to Kubelet
	CommitChanges() error
}
