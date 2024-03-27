package configurator

import "github.com/aws/eks-anywhere-packages/credentialproviderpackage/pkg/constants"

type Configurator interface {
	// Initialize Handles node specific configuration depending on OS
	Initialize(config constants.CredentialProviderConfigOptions)

	// UpdateAWSCredentials Handles AWS Credential Setup
	UpdateAWSCredentials(sourcePath, profile string) error

	// UpdateCredentialProvider Handles Credential Provider Setup
	UpdateCredentialProvider(profile string) error

	// CommitChanges Applies changes to Kubelet
	CommitChanges() error
}
