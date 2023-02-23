package constants

const (
	// Credential Provider constants
	DefaultImagePattern  = "*.dkr.ecr.*.amazonaws.com"
	DefaultCacheDuration = "30m"
	CredProviderFile     = "credential-provider-config.yaml"

	// Aws Credentials
	CredSrcPath   = "/secrets/aws-creds/config"
	Profile       = "eksa-packages"
	CredWatchData = "/secrets/aws-creds/..data"
	CredWatchPath = "/secrets/aws-creds/"

	// BottleRocket
	SocketPath = "/run/api.sock"

	// Linux
	BinPath          = "/eksa-binaries/"
	BasePath         = "/eksa-packages/"
	CredOutFile      = "aws-creds"
	MountedExtraArgs = "/node-files/kubelet-extra-args"

	// Binaries
	ECRCredProviderBinary = "ecr-credential-provider"
	IAMRolesSigningBinary = "aws_signing_helper"
)

type OSType string

const (
	Docker       OSType = "docker"
	Ubuntu              = "ubuntu"
	Redhat              = "redhat"
	BottleRocket        = "bottlerocket"
)

type CredentialProviderConfigOptions struct {
	ImagePatterns        []string
	DefaultCacheDuration string
}
