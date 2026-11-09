package linux

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/eks-anywhere-packages/credentialproviderpackage/internal/test"
	"github.com/aws/eks-anywhere-packages/credentialproviderpackage/pkg/constants"
)

func Test_linuxOS_updateKubeletArguments(t *testing.T) {
	testDir, _ := test.NewWriter(t)
	dir := testDir + "/"
	type fields struct {
		profile             string
		extraArgsPath       string
		legacyExtraArgsPath string
		basePath            string
		config              constants.CredentialProviderConfigOptions
	}
	type args struct {
		line string
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		outputConfigPath string
		configWantPath   string
		k8sVersion       string
		want             string
	}{
		{
			name: "test empty string",
			fields: fields{
				profile:             "eksa-packages",
				extraArgsPath:       dir,
				legacyExtraArgsPath: dir,
				basePath:            dir,
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns:        []string{constants.DefaultImagePattern},
					DefaultCacheDuration: constants.DefaultCacheDuration,
				},
			},
			args:             args{line: ""},
			outputConfigPath: dir + "/" + credProviderFile,
			configWantPath:   "testdata/expected-config.yaml",
			want:             fmt.Sprintf(" --image-credential-provider-config=%s%s", dir, credProviderFile),
		},
		{
			name: "test multiple match patterns",
			fields: fields{
				profile:             "eksa-packages",
				extraArgsPath:       dir,
				legacyExtraArgsPath: dir,
				basePath:            dir,
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns: []string{
						"1234567.dkr.ecr.us-east-1.amazonaws.com",
						"7654321.dkr.ecr.us-west-2.amazonaws.com",
					},
					DefaultCacheDuration: constants.DefaultCacheDuration,
				},
			},
			args:             args{line: ""},
			outputConfigPath: dir + "/" + credProviderFile,
			configWantPath:   "testdata/expected-config-multiple-patterns.yaml",
			want:             fmt.Sprintf(" --image-credential-provider-config=%s%s", dir, credProviderFile),
		},
		{
			name: "skip credential provider if already provided",
			fields: fields{
				profile:             "eksa-packages",
				extraArgsPath:       dir,
				legacyExtraArgsPath: dir,
				basePath:            dir,
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns:        []string{constants.DefaultImagePattern},
					DefaultCacheDuration: constants.DefaultCacheDuration,
				},
			},
			args:             args{line: " --image-credential-provider-config=blah"},
			outputConfigPath: dir + "/" + credProviderFile,
			configWantPath:   "",
			want:             "",
		},
		{
			name: "test alpha api",
			fields: fields{
				profile:             "eksa-packages",
				extraArgsPath:       dir,
				legacyExtraArgsPath: dir,
				basePath:            dir,
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns:        []string{constants.DefaultImagePattern},
					DefaultCacheDuration: constants.DefaultCacheDuration,
				},
			},
			args:             args{line: ""},
			outputConfigPath: dir + "/" + credProviderFile,
			configWantPath:   "testdata/expected-config-alpha.yaml",
			k8sVersion:       "v1.25",
			want:             fmt.Sprintf(" --image-credential-provider-config=%s%s", dir, credProviderFile),
		},
		{
			name: "test v1 api 1.27",
			fields: fields{
				profile:             "eksa-packages",
				extraArgsPath:       dir,
				legacyExtraArgsPath: dir,
				basePath:            dir,
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns:        []string{constants.DefaultImagePattern},
					DefaultCacheDuration: constants.DefaultCacheDuration,
				},
			},
			args:             args{line: ""},
			outputConfigPath: dir + "/" + credProviderFile,
			configWantPath:   "testdata/expected-config.yaml",
			k8sVersion:       "v1.27",
			want:             fmt.Sprintf(" --image-credential-provider-config=%s%s", dir, credProviderFile),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &linuxOS{
				profile:             tt.fields.profile,
				extraArgsPath:       tt.fields.extraArgsPath,
				legacyExtraArgsPath: tt.fields.legacyExtraArgsPath,
				basePath:            tt.fields.basePath,
				config:              tt.fields.config,
			}
			t.Setenv("K8S_VERSION", tt.k8sVersion)

			if got := c.updateKubeletArguments(tt.args.line); got != tt.want {
				t.Errorf("updateKubeletArguments() = %v, want %v", got, tt.want)
			}
			if tt.configWantPath != "" {
				test.AssertFilesEquals(t, tt.outputConfigPath, tt.configWantPath)
			}
		})
	}
}

func Test_linuxOS_UpdateAWSCredentials(t *testing.T) {
	testDir, _ := test.NewWriter(t)
	dir := testDir + "/"
	type fields struct {
		profile             string
		extraArgsPath       string
		legacyExtraArgsPath string
		basePath            string
		config              constants.CredentialProviderConfigOptions
	}
	type args struct {
		sourcePath string
		profile    string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "simple credential move",
			fields: fields{
				profile:             "eksa-packages",
				extraArgsPath:       dir,
				legacyExtraArgsPath: dir,
				basePath:            dir,
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns:        []string{constants.DefaultImagePattern},
					DefaultCacheDuration: constants.DefaultCacheDuration,
				},
			},
			args: args{
				sourcePath: "testdata/testcreds",
				profile:    "eksa-packages",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dstFile := tt.fields.basePath + credOutFile
			c := &linuxOS{
				profile:             tt.fields.profile,
				extraArgsPath:       tt.fields.extraArgsPath,
				legacyExtraArgsPath: tt.fields.legacyExtraArgsPath,
				basePath:            tt.fields.basePath,
				config:              tt.fields.config,
			}
			if err := c.UpdateAWSCredentials(tt.args.sourcePath, tt.args.profile); (err != nil) != tt.wantErr {
				t.Errorf("UpdateAWSCredentials() error = %v, wantErr %v", err, tt.wantErr)
			}
			info, err := os.Stat(dstFile)
			if err != nil {
				t.Errorf("Failed to open destination file")
			}
			if info.Mode().Perm() != os.FileMode(0o600) {
				t.Errorf("Credential file not saved with correct permission")
			}

			if err != nil {
				t.Errorf("Failed to set file back to readable")
			}
			expectedCreds, err := os.ReadFile(tt.args.sourcePath)
			if err != nil {
				t.Errorf("Failed to read source credential file")
			}

			actualCreds, err := os.ReadFile(dstFile)
			if err != nil {
				t.Errorf("Failed to read created credential file")
			}
			assert.Equal(t, expectedCreds, actualCreds)
		})
	}
}

func Test_linuxOS_UpdateAWSCredentialsReplacesFile(t *testing.T) {
	dir := t.TempDir() + "/"
	src1 := filepath.Join(t.TempDir(), "creds-1")
	src2 := filepath.Join(t.TempDir(), "creds-2")
	assert.NoError(t, os.WriteFile(src1, []byte("first"), 0o644))
	assert.NoError(t, os.WriteFile(src2, []byte("second"), 0o644))
	dstFile := dir + credOutFile
	c := &linuxOS{basePath: dir}

	assert.NoError(t, c.UpdateAWSCredentials(src1, "eksa-packages"))
	before, err := os.Stat(dstFile)
	assert.NoError(t, err)
	held, err := os.Open(dstFile)
	assert.NoError(t, err)
	defer held.Close()

	assert.NoError(t, c.UpdateAWSCredentials(src2, "eksa-packages"))
	after, err := os.Stat(dstFile)
	assert.NoError(t, err)

	assert.False(t, os.SameFile(before, after), "update should replace the credential file, not rewrite it in place")
	assert.Equal(t, os.FileMode(0o600), after.Mode().Perm())
	got, err := os.ReadFile(dstFile)
	assert.NoError(t, err)
	assert.Equal(t, "second", string(got))
	old, err := io.ReadAll(held)
	assert.NoError(t, err)
	assert.Equal(t, "first", string(old), "a file opened before the update should not see the new credentials")

	entries, err := os.ReadDir(dir)
	assert.NoError(t, err)
	assert.Len(t, entries, 1, "no temporary files should be left behind")
}

func Test_copyWithPermissons(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src")
	assert.NoError(t, os.WriteFile(src, []byte("content"), 0o644))

	t.Run("sets the requested permission", func(t *testing.T) {
		for _, perm := range []os.FileMode{0o600, 0o700} {
			dst := filepath.Join(t.TempDir(), "dst")
			assert.NoError(t, copyWithPermissons(src, dst, perm))
			info, err := os.Stat(dst)
			assert.NoError(t, err)
			assert.Equal(t, perm, info.Mode().Perm())
			got, err := os.ReadFile(dst)
			assert.NoError(t, err)
			assert.Equal(t, "content", string(got))
		}
	})

	t.Run("replaces an existing file with looser permissions", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "dst")
		assert.NoError(t, os.WriteFile(dst, []byte("old"), 0o644))
		assert.NoError(t, copyWithPermissons(src, dst, 0o600))
		info, err := os.Stat(dst)
		assert.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})

	t.Run("missing source leaves nothing behind", func(t *testing.T) {
		dir := t.TempDir()
		assert.Error(t, copyWithPermissons(filepath.Join(dir, "missing"), filepath.Join(dir, "dst"), 0o600))
		entries, err := os.ReadDir(dir)
		assert.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("missing destination directory", func(t *testing.T) {
		assert.Error(t, copyWithPermissons(src, filepath.Join(t.TempDir(), "missing", "dst"), 0o600))
	})

	t.Run("failed rename removes the temporary file", func(t *testing.T) {
		dir := t.TempDir()
		dst := filepath.Join(dir, "dst")
		// A non-empty directory at dst makes the rename fail.
		assert.NoError(t, os.MkdirAll(filepath.Join(dst, "child"), 0o755))
		assert.Error(t, copyWithPermissons(src, dst, 0o600))
		entries, err := os.ReadDir(dir)
		assert.NoError(t, err)
		assert.Len(t, entries, 1)
		assert.Equal(t, "dst", entries[0].Name())
	})
}

func Test_linuxOS_Initialize(t *testing.T) {
	type fields struct {
		profile             string
		extraArgsPath       string
		legacyExtraArgsPath string
		basePath            string
		config              constants.CredentialProviderConfigOptions
	}
	type args struct {
		config constants.CredentialProviderConfigOptions
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "simple initialization",
			args: args{
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns:        []string{constants.DefaultImagePattern},
					DefaultCacheDuration: constants.DefaultCacheDuration,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewLinuxConfigurator()
			c.Initialize(tt.args.config)
			assert.Equal(t, c.config, tt.args.config)
		})
	}
}
