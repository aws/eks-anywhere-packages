package linux

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"credential-provider/internal/test"
	"credential-provider/pkg/constants"
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
		want             string
	}{
		{
			name: "test empty string",
			fields: fields{
				profile:       "eksa-packages",
				extraArgsPath: dir,
				basePath:      dir,
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns:        constants.ImagePattern,
					DefaultCacheDuration: constants.CacheDuration,
				},
			},
			args:             args{line: ""},
			outputConfigPath: dir + "/" + constants.CredProviderFile,
			configWantPath:   "testdata/expected-config.yaml",
			want: fmt.Sprintf(" --feature-gates=KubeletCredentialProviders=true "+
				"--image-credential-provider-config=%s%s", dir, constants.CredProviderFile),
		},
		{
			name: "skip credential provider if already provided",
			fields: fields{
				profile:       "eksa-packages",
				extraArgsPath: dir,
				basePath:      dir,
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns:        constants.ImagePattern,
					DefaultCacheDuration: constants.CacheDuration,
				},
			},
			args:             args{line: " --feature-gates=KubeletCredentialProviders=true"},
			outputConfigPath: dir + "/" + constants.CredProviderFile,
			configWantPath:   "testdata/expected-config.yaml",
			want:             fmt.Sprintf(" --image-credential-provider-config=%s%s", dir, constants.CredProviderFile),
		},
		{
			name: "skip both cred provider and feature gate if provided",
			fields: fields{
				profile:       "eksa-packages",
				extraArgsPath: dir,
				basePath:      dir,
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns:        constants.ImagePattern,
					DefaultCacheDuration: constants.CacheDuration,
				},
			},
			args:             args{line: " --feature-gates=KubeletCredentialProviders=false --image-credential-provider-config=blah"},
			outputConfigPath: dir + "/" + constants.CredProviderFile,
			configWantPath:   "",
			want:             "",
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
					ImagePatterns:        constants.ImagePattern,
					DefaultCacheDuration: constants.CacheDuration,
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
			dstFile := tt.fields.basePath + constants.CredOutFile
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
		in0    string
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
				in0: "",
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns:        constants.ImagePattern,
					DefaultCacheDuration: constants.CacheDuration,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewLinuxConfigurator()
			c.Initialize(tt.args.in0, tt.args.config)
			assert.Equal(t, c.config, tt.args.config)
		})
	}
}
