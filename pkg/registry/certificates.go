package registry

import (
	"crypto/x509"
	"fmt"
	"os"
	"path"
	"path/filepath"
)

const (
	registryConfigPath = "/tmp/config/registry"
	certFile           = "ca.crt"
	insecureFile       = "insecure"
)

func GetManagementClusterCertificate() (certificates *x509.CertPool, err error) {
	return GetClusterCertificate(os.Getenv("CLUSTER_NAME"))
}

func GetRegistryInsecure(clusterName string) bool {
	caFile := path.Join(registryConfigPath, clusterName, insecureFile)
	if _, err := os.Stat(caFile); err != nil {
		return false
	}
	return true
}

func GetClusterCertificateFileName(clusterName string) string {
	caFile := path.Join(registryConfigPath, clusterName, certFile)
	if _, err := os.Stat(caFile); err != nil {
		return ""
	}
	return caFile
}

func GetClusterCertificate(clusterName string) (certificates *x509.CertPool, err error) {
	caFile := GetClusterCertificateFileName(clusterName)
	if caFile == "" {
		return nil, nil
	}
	return GetCertificates(caFile)
}

// GetCertificates get X509 certificates.
func GetCertificates(certFile string) (certificates *x509.CertPool, err error) {
	if len(certFile) < 1 {
		return nil, nil
	}
	fileContents, err := os.ReadFile(filepath.Clean(certFile))
	if err != nil {
		return nil, fmt.Errorf("error reading certificate file <%s>: %v", certFile, err)
	}
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(fileContents)

	return certPool, nil
}
