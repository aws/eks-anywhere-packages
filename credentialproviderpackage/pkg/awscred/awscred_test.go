package awscred

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/aws/eks-anywhere-packages/credentialproviderpackage/internal/test"
)

func Test_generateAwsConfigSecret(t *testing.T) {
	testDir, _ := test.NewWriter(t)
	dir := testDir + "/"
	err := createTestFiles(dir)
	wantString := fmt.Sprintf(
		`
[default]
aws_access_key_id=abc
aws_secret_access_key=def
region=us-east-3
`)
	if err != nil {
		t.Errorf("Failed to create test files")
	}
	type args struct {
		accessKeyPath       string
		secretAccessKeyPath string
		regionPath          string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "test create config",
			args: args{
				accessKeyPath:       dir + "accessKey",
				secretAccessKeyPath: dir + "secretAccessKey",
				regionPath:          dir + "region",
			},
			want:    wantString,
			wantErr: false,
		},
		{
			name: "nonexistent path accesskey",
			args: args{
				accessKeyPath:       dir + "wrongPath",
				secretAccessKeyPath: dir + "secretAccessKey",
				regionPath:          dir + "region",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "nonexistent path secretAccesskey",
			args: args{
				accessKeyPath:       dir + "accessKey",
				secretAccessKeyPath: dir + "wrongPath",
				regionPath:          dir + "region",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "nonexistent path region",
			args: args{
				accessKeyPath:       dir + "accessKey",
				secretAccessKeyPath: dir + "secretAccessKey",
				regionPath:          dir + "wrongPath",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "correctly trim secretKey",
			args: args{
				accessKeyPath:       dir + "accessKey",
				secretAccessKeyPath: dir + "secretAccessKeyWithQuote",
				regionPath:          dir + "region",
			},
			want:    wantString,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := generateAwsConfigSecret(tt.args.accessKeyPath, tt.args.secretAccessKeyPath, tt.args.regionPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("generateAwsConfigSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("generateAwsConfigSecret() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func createTestFiles(baseDir string) error {
	writeMap := map[string]string{
		"accessKey":                "abc",
		"secretAccessKey":          "def",
		"region":                   "us-east-3",
		"secretAccessKeyWithQuote": "'def'",
	}

	for filePath, data := range writeMap {
		err := ioutil.WriteFile(baseDir+filePath, []byte(data), 0o600)
		if err != nil {
			return err
		}
	}
	return nil
}
