package driver

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/kube"
	"helm.sh/helm/v4/pkg/kube/fake"
	"helm.sh/helm/v4/pkg/provenance"
	"helm.sh/helm/v4/pkg/registry"
	release "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage"
	storagedriver "helm.sh/helm/v4/pkg/storage/driver"
)

type hookWaitKube struct {
	fake.PrintingKubeClient
}

func (k *hookWaitKube) GetWaiterWithOptions(strategy kube.WaitStrategy, opts ...kube.WaitOption) (kube.Waiter, error) {
	if strategy != kube.HookOnlyStrategy {
		return nil, fmt.Errorf("unexpected resource wait strategy %q", strategy)
	}
	return k.PrintingKubeClient.GetWaiterWithOptions(strategy, opts...)
}

func TestHelmReleaseLifecyclePreservesClientSideApply(t *testing.T) {
	cfg := action.NewConfiguration()
	cfg.KubeClient = &hookWaitKube{PrintingKubeClient: fake.PrintingKubeClient{Out: io.Discard}}
	cfg.Releases = storage.Init(storagedriver.NewMemory())
	cfg.Capabilities = common.DefaultCapabilities
	driver := &helmDriver{cfg: cfg}
	ch := &chart.Chart{Metadata: &chart.Metadata{Name: "test", Version: "1.0.0", APIVersion: "v2"}}
	install := driver.newInstallAction()
	install.ReleaseName, install.Namespace = "test", "default"
	created, err := install.RunWithContext(ctx, ch, map[string]interface{}{"value": "original"})
	require.NoError(t, err)
	require.Equal(t, "csa", created.(*release.Release).ApplyMethod)
	legacy := created.(*release.Release)
	legacy.ApplyMethod = ""
	require.NoError(t, cfg.Releases.Update(legacy))
	updated, err := driver.newUpgradeAction().RunWithContext(ctx, "test", ch, map[string]interface{}{"value": "updated"})
	require.NoError(t, err)
	require.Equal(t, "csa", updated.(*release.Release).ApplyMethod)
	stored, err := cfg.Releases.Last("test")
	require.NoError(t, err)
	require.Equal(t, "updated", stored.(*release.Release).Config["value"])
	_, err = driver.newUninstallAction().Run("test")
	require.NoError(t, err)
}

func TestHelmRegistryTLSAndCredentials(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "test" || password != "dummy" {
			w.Header().Set("WWW-Authenticate", `Basic realm="registry-test"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"test","tags":["1.0.0"]}`))
	}))
	defer server.Close()
	host := strings.TrimPrefix(server.URL, "https://")
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{
		Type: "CERTIFICATE", Bytes: server.Certificate().Raw,
	}), 0o600))

	for _, tc := range []struct {
		name     string
		ca       string
		password string
		wantErr  bool
		insecure bool
	}{
		{name: "trusted authenticated registry", ca: caFile, password: "dummy"},
		{name: "CA with explicit insecure option", ca: caFile, password: "dummy", insecure: true},
		{name: "explicit insecure option without CA", password: "dummy", insecure: true},
		{name: "untrusted registry rejected", password: "dummy", wantErr: true},
		{name: "wrong credential rejected", ca: caFile, password: "wrong", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := cli.New()
			settings.RegistryConfig = filepath.Join(t.TempDir(), "config.json")
			config, err := json.Marshal(map[string]interface{}{
				"auths": map[string]interface{}{
					host: map[string]string{"auth": base64.StdEncoding.EncodeToString([]byte("test:" + tc.password))},
				},
			})
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(settings.RegistryConfig, config, 0o600))
			client, err := newRegistryClientWithTLS("", "", tc.ca, tc.insecure, settings)
			require.NoError(t, err)
			tags, err := client.Tags(host + "/test")
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, []string{"1.0.0"}, tags)
		})
	}

	_, err := newRegistryClientWithTLS("", "", filepath.Join(t.TempDir(), "missing.pem"), false, cli.New())
	require.Error(t, err)
	for _, ca := range []string{"invalid PEM", ""} {
		caFile := filepath.Join(t.TempDir(), "invalid-ca.pem")
		require.NoError(t, os.WriteFile(caFile, []byte(ca), 0o600))
		for _, insecure := range []bool{false, true} {
			_, err := newRegistryClientWithTLS("", "", caFile, insecure, cli.New())
			require.Error(t, err)
		}
	}
}

func registryClientCertificate(t *testing.T) (string, string, *x509.CertPool) {
	t.Helper()
	caPublic, caKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test-ca"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, caPublic, caKey)
	require.NoError(t, err)
	ca, err = x509.ParseCertificate(caDER)
	require.NoError(t, err)
	public, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	certificate := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "test-client"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, certificate, ca, public, caKey)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "client.pem"), filepath.Join(dir, "client-key.pem")
	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600))
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	return certFile, keyFile, pool
}

func TestHelmRegistryMutualTLS(t *testing.T) {
	certFile, keyFile, pool := registryClientCertificate(t)
	wrongCert, wrongKey, _ := registryClientCertificate(t)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"test","tags":["1.0.0"]}`))
	}))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool}
	server.StartTLS()
	defer server.Close()
	caFile := filepath.Join(t.TempDir(), "server-ca.pem")
	require.NoError(t, os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{
		Type: "CERTIFICATE", Bytes: server.Certificate().Raw,
	}), 0o600))

	for _, tc := range []struct {
		name            string
		cert, key       string
		wantConfigError bool
		wantAuthError   bool
		insecure        bool
	}{
		{name: "trusted client certificate", cert: certFile, key: keyFile},
		{name: "CA and explicit insecure still authenticate client", cert: certFile, key: keyFile, insecure: true},
		{name: "client certificate required", wantAuthError: true},
		{name: "untrusted client certificate", cert: wrongCert, key: wrongKey, wantAuthError: true},
		{name: "insecure option does not bypass client authentication", cert: wrongCert, key: wrongKey, insecure: true, wantAuthError: true},
		{name: "certificate without key", cert: certFile, wantConfigError: true},
		{name: "key without certificate", key: keyFile, wantConfigError: true},
		{name: "mismatched key pair", cert: certFile, key: wrongKey, wantConfigError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := cli.New()
			settings.RegistryConfig = filepath.Join(t.TempDir(), "config.json")
			client, err := newRegistryClient(tc.cert, tc.key, caFile, tc.insecure, settings)
			if tc.wantConfigError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			tags, err := client.Tags(strings.TrimPrefix(server.URL, "https://") + "/test")
			if tc.wantAuthError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, []string{"1.0.0"}, tags)
		})
	}
	_, err := newRegistryClient(certFile, "", "", true, cli.New())
	require.Error(t, err)
}

func TestHelmRegistryPullPinnedDigest(t *testing.T) {
	metadata := []byte("apiVersion: v2\nname: test\nversion: 1.0.0\n")
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "test/Chart.yaml", Mode: 0o600, Size: int64(len(metadata))}))
	_, err := tw.Write(metadata)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	chartBytes := archive.Bytes()
	config := []byte(`{"apiVersion":"v2","name":"test","version":"1.0.0"}`)
	manifest := ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config: ocispec.Descriptor{
			MediaType: registry.ConfigMediaType, Digest: digest.FromBytes(config), Size: int64(len(config)),
		},
		Layers: []ocispec.Descriptor{{
			MediaType: registry.ChartLayerMediaType, Digest: digest.FromBytes(chartBytes), Size: int64(len(chartBytes)),
		}},
	}
	manifest.SchemaVersion = 2
	manifestBytes, err := json.Marshal(manifest)
	require.NoError(t, err)
	manifestDigest := digest.FromBytes(manifestBytes)

	for _, tamper := range []bool{false, true} {
		t.Run(map[bool]string{false: "pinned chart", true: "corrupt blob rejected"}[tamper], func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v2/":
					_, _ = w.Write([]byte(`{}`))
				case "/v2/test/manifests/" + manifestDigest.String():
					w.Header().Set("Content-Type", ocispec.MediaTypeImageManifest)
					w.Header().Set("Docker-Content-Digest", manifestDigest.String())
					_, _ = w.Write(manifestBytes)
				case "/v2/test/blobs/" + digest.FromBytes(config).String():
					_, _ = w.Write(config)
				case "/v2/test/blobs/" + digest.FromBytes(chartBytes).String():
					if tamper {
						_, _ = w.Write([]byte("corrupt blob"))
					} else {
						_, _ = w.Write(chartBytes)
					}
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			caFile := filepath.Join(t.TempDir(), "ca.pem")
			require.NoError(t, os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{
				Type: "CERTIFICATE", Bytes: server.Certificate().Raw,
			}), 0o600))
			settings := cli.New()
			settings.RegistryConfig = filepath.Join(t.TempDir(), "config.json")
			client, err := newRegistryClientWithTLS("", "", caFile, false, settings)
			require.NoError(t, err)
			pulled, err := client.Pull(strings.TrimPrefix(server.URL, "https://") + "/test@" + manifestDigest.String())
			if tamper {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, chartBytes, pulled.Chart.Data)
			chart, err := loader.LoadArchive(bytes.NewReader(pulled.Chart.Data))
			require.NoError(t, err)
			require.Equal(t, "test", chart.Metadata.Name)
		})
	}
}

func TestHelmProvenanceRejectsTamperedChart(t *testing.T) {
	entity, err := openpgp.NewEntity("test", "", "test@example.com", &packet.Config{RSABits: 2048})
	require.NoError(t, err)
	signer := &provenance.Signatory{Entity: entity, KeyRing: openpgp.EntityList{entity}}
	archive := []byte("dummy chart archive")
	metadata := []byte("apiVersion: v2\nname: test\nversion: 1.0.0\n")
	signature, err := signer.ClearSign(archive, "test-1.0.0.tgz", metadata)
	require.NoError(t, err)
	_, err = signer.Verify(archive, []byte(signature), "test-1.0.0.tgz")
	require.NoError(t, err)
	_, err = signer.Verify([]byte("tampered chart archive"), []byte(signature), "test-1.0.0.tgz")
	require.Error(t, err)
}
