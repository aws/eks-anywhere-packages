package constants

const (
	// Credential Provider constants
	DefaultImagePattern  = "*.dkr.ecr.*.amazonaws.com"
	DefaultCacheDuration = "30m"
)

type CredentialProviderConfigOptions struct {
	ImagePatterns        []string
	DefaultCacheDuration string
}
