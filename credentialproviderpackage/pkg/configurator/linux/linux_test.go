package linux

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/eks-anywhere-packages/credentialproviderpackage/internal/test"
	"github.com/aws/eks-anywhere-packages/credentialproviderpackage/pkg/constants"
)

func Test_linuxOS_updateKubeletArguments(t *testing.T) {
	testDir, _ := test.NewWriter(t)
	dir := testDir + "/"
	type fields struct {
		profile       string
		extraArgsPath string
		basePath      string
		config        constants.CredentialProviderConfigOptions
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
				profile:       "eksa-packages",
				extraArgsPath: dir,
				basePath:      dir,
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns:        []string{constants.DefaultImagePattern},
					DefaultCacheDuration: constants.DefaultCacheDuration,
				},
			},
			args:             args{line: ""},
			outputConfigPath: dir + "/" + credProviderFile,
			configWantPath:   "testdata/expected-config.yaml",
			want: fmt.Sprintf(" --feature-gates=KubeletCredentialProviders=true "+
				"--image-credential-provider-config=%s%s", dir, credProviderFile),
		},
		{
			name: "test multiple match patterns",
			fields: fields{
				profile:       "eksa-packages",
				extraArgsPath: dir,
				basePath:      dir,
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns: []string{"1234567.dkr.ecr.us-east-1.amazonaws.com",
						"7654321.dkr.ecr.us-west-2.amazonaws.com"},
					DefaultCacheDuration: constants.DefaultCacheDuration,
				},
			},
			args:             args{line: ""},
			outputConfigPath: dir + "/" + credProviderFile,
			configWantPath:   "testdata/expected-config-multiple-patterns.yaml",
			want: fmt.Sprintf(" --feature-gates=KubeletCredentialProviders=true "+
				"--image-credential-provider-config=%s%s", dir, credProviderFile),
		},
		{
			name: "skip credential provider if already provided",
			fields: fields{
				profile:       "eksa-packages",
				extraArgsPath: dir,
				basePath:      dir,
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns:        []string{constants.DefaultImagePattern},
					DefaultCacheDuration: constants.DefaultCacheDuration,
				},
			},
			args:             args{line: " --feature-gates=KubeletCredentialProviders=true"},
			outputConfigPath: dir + "/" + credProviderFile,
			configWantPath:   "testdata/expected-config.yaml",
			want:             fmt.Sprintf(" --image-credential-provider-config=%s%s", dir, credProviderFile),
		},
		{
			name: "skip both cred provider and feature gate if provided",
			fields: fields{
				profile:       "eksa-packages",
				extraArgsPath: dir,
				basePath:      dir,
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns:        []string{constants.DefaultImagePattern},
					DefaultCacheDuration: constants.DefaultCacheDuration,
				},
			},
			args:             args{line: " --feature-gates=KubeletCredentialProviders=false --image-credential-provider-config=blah"},
			outputConfigPath: dir + "/" + credProviderFile,
			configWantPath:   "",
			want:             "",
		},
		{
			name: "test alpha api",
			fields: fields{
				profile:       "eksa-packages",
				extraArgsPath: dir,
				basePath:      dir,
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns:        []string{constants.DefaultImagePattern},
					DefaultCacheDuration: constants.DefaultCacheDuration,
				},
			},
			args:             args{line: ""},
			outputConfigPath: dir + "/" + credProviderFile,
			configWantPath:   "testdata/expected-config-alpha.yaml",
			k8sVersion:       "v1.23",
			want: fmt.Sprintf(" --feature-gates=KubeletCredentialProviders=true "+
				"--image-credential-provider-config=%s%s", dir, credProviderFile),
		},
		{
			name: "test beta api",
			fields: fields{
				profile:       "eksa-packages",
				extraArgsPath: dir,
				basePath:      dir,
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns:        []string{constants.DefaultImagePattern},
					DefaultCacheDuration: constants.DefaultCacheDuration,
				},
			},
			args:             args{line: ""},
			outputConfigPath: dir + "/" + credProviderFile,
			configWantPath:   "testdata/expected-config-beta.yaml",
			k8sVersion:       "v1.25",
			want: fmt.Sprintf(" --feature-gates=KubeletCredentialProviders=true "+
				"--image-credential-provider-config=%s%s", dir, credProviderFile),
		},
		{
			name: "test v1 api",
			fields: fields{
				profile:       "eksa-packages",
				extraArgsPath: dir,
				basePath:      dir,
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns:        []string{constants.DefaultImagePattern},
					DefaultCacheDuration: constants.DefaultCacheDuration,
				},
			},
			args:             args{line: ""},
			outputConfigPath: dir + "/" + credProviderFile,
			configWantPath:   "testdata/expected-config.yaml",
			k8sVersion:       "v1.27",
			want: fmt.Sprintf(" --feature-gates=KubeletCredentialProviders=true "+
				"--image-credential-provider-config=%s%s", dir, credProviderFile),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &linuxOS{
				profile:       tt.fields.profile,
				extraArgsPath: tt.fields.extraArgsPath,
				basePath:      tt.fields.basePath,
				config:        tt.fields.config,
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
		profile       string
		extraArgsPath string
		basePath      string
		config        constants.CredentialProviderConfigOptions
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
				profile:       "eksa-packages",
				extraArgsPath: dir,
				basePath:      dir,
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
				profile:       tt.fields.profile,
				extraArgsPath: tt.fields.extraArgsPath,
				basePath:      tt.fields.basePath,
				config:        tt.fields.config,
			}
			if err := c.UpdateAWSCredentials(tt.args.sourcePath, tt.args.profile); (err != nil) != tt.wantErr {
				t.Errorf("UpdateAWSCredentials() error = %v, wantErr %v", err, tt.wantErr)
			}
			info, err := os.Stat(dstFile)
			if err != nil {
				t.Errorf("Failed to open destination file")
			}
			if info.Mode().Perm() != os.FileMode(0600) {
				t.Errorf("Credential file not saved with correct permission")
			}

			if err != nil {
				t.Errorf("Failed to set file back to readable")
			}
			expectedCreds, err := ioutil.ReadFile(tt.args.sourcePath)
			if err != nil {
				t.Errorf("Failed to read source credential file")
			}

			actualCreds, err := ioutil.ReadFile(dstFile)
			if err != nil {
				t.Errorf("Failed to read created credential file")
			}
			assert.Equal(t, expectedCreds, actualCreds)
		})
	}
}

func Test_linuxOS_Initialize(t *testing.T) {
	type fields struct {
		profile       string
		extraArgsPath string
		basePath      string
		config        constants.CredentialProviderConfigOptions
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
