package bottlerocket

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"credential-provider/pkg/constants"
)

type response struct {
	statusCode   int
	expectedBody []byte
	responseMsg  string
}

func Test_bottleRocket_CommitChanges(t *testing.T) {
	type fields struct {
		client  http.Client
		baseURL string
		config  constants.CredentialProviderConfigOptions
	}

	tests := []struct {
		name     string
		fields   fields
		wantErr  bool
		response response
		expected string
	}{
		{
			name: "test success",
			fields: fields{
				client: http.Client{},
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns:        []string{constants.DefaultImagePattern},
					DefaultCacheDuration: constants.DefaultCacheDuration,
				},
			},
			wantErr: false,
			response: response{
				statusCode:  http.StatusOK,
				responseMsg: "",
			},
		},
		{
			name: "test fail",
			fields: fields{
				client: http.Client{},
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns:        []string{constants.DefaultImagePattern},
					DefaultCacheDuration: constants.DefaultCacheDuration,
				},
			},
			wantErr: true,
			response: response{
				statusCode:  http.StatusNotFound,
				responseMsg: "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.response.statusCode)
				fmt.Fprintf(w, tt.response.responseMsg)
			}))
			b := &bottleRocket{
				client:  tt.fields.client,
				baseURL: svr.URL + "/",
				config:  tt.fields.config,
			}
			if err := b.CommitChanges(); (err != nil) != tt.wantErr {
				t.Errorf("UpdateAWSCredentials() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_bottleRocket_UpdateAWSCredentials(t *testing.T) {
	file, err := os.Open("testdata/testcreds")
	if err != nil {
		t.Errorf("Failed to open testcreds")
	}
	content, err := io.ReadAll(file)
	if err != nil {
		t.Errorf("Failed to read testcreds")
	}
	encodedSecret := base64.StdEncoding.EncodeToString(content)
	expectedBody := fmt.Sprintf("{\"aws\":{\"config\":\"%s\",\"profile\":\"eksa-packages\",\"region\":\"\"}}", encodedSecret)

	type fields struct {
		client  http.Client
		baseURL string
		config  constants.CredentialProviderConfigOptions
	}
	type args struct {
		path    string
		profile string
	}
	tests := []struct {
		name           string
		fields         fields
		args           args
		patchResponse  response
		commitResponse response
		wantErr        bool
	}{
		{
			name: "working credential update",
			fields: fields{
				client: http.Client{},
				config: constants.CredentialProviderConfigOptions{},
			},
			args: args{
				path:    "testdata/testcreds",
				profile: "eksa-packages",
			},
			patchResponse: response{
				statusCode:   http.StatusNoContent,
				expectedBody: []byte(expectedBody),
				responseMsg:  "",
			},
			commitResponse: response{
				statusCode:  http.StatusOK,
				responseMsg: "",
			},
			wantErr: false,
		},
		{
			name: "commit credentials failed",
			fields: fields{
				client: http.Client{},
				config: constants.CredentialProviderConfigOptions{},
			},
			args: args{
				path:    "testdata/testcreds",
				profile: "eksa-packages",
			},
			patchResponse: response{
				statusCode:   http.StatusNoContent,
				expectedBody: []byte(expectedBody),
				responseMsg:  "",
			},
			commitResponse: response{
				statusCode:  http.StatusNotFound,
				responseMsg: "",
			},
			wantErr: true,
		},
		{
			name: "failed to patch data",
			fields: fields{
				client: http.Client{},
				config: constants.CredentialProviderConfigOptions{},
			},
			args: args{
				path:    "testdata/testcreds",
				profile: "eksa-packages",
			},
			patchResponse: response{
				statusCode:   http.StatusNotFound,
				expectedBody: []byte(expectedBody),
				responseMsg:  "",
			},
			commitResponse: response{
				statusCode:  http.StatusOK,
				responseMsg: "",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodPatch {
						validatePatchRequest(w, r, t, tt.patchResponse)
					} else if r.Method == http.MethodPost {
						w.WriteHeader(tt.commitResponse.statusCode)
						fmt.Fprintf(w, tt.commitResponse.responseMsg)
					} else {
						t.Errorf("Recieved unexected request %v", r.Method)
					}
				}),
			)
			b := &bottleRocket{
				client:  tt.fields.client,
				baseURL: svr.URL + "/",
				config:  tt.fields.config,
			}
			if err := b.UpdateAWSCredentials(tt.args.path, tt.args.profile); (err != nil) != tt.wantErr {
				t.Errorf("UpdateAWSCredentials() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_bottleRocket_UpdateCredentialProvider(t *testing.T) {
	type fields struct {
		client  http.Client
		baseURL string
		config  constants.CredentialProviderConfigOptions
	}

	tests := []struct {
		name          string
		fields        fields
		patchResponse response
		wantErr       bool
	}{
		{
			name: "default credential provider",
			fields: fields{
				client: http.Client{},
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns:        []string{constants.DefaultImagePattern},
					DefaultCacheDuration: constants.DefaultCacheDuration,
				},
			},
			patchResponse: response{
				statusCode:   http.StatusNoContent,
				expectedBody: []byte("{\"kubernetes\":{\"credential-providers\":{\"ecr-credential-provider\":{\"cache-duration\":\"30m\",\"enabled\":true,\"image-patterns\":[\"*.dkr.ecr.*.amazonaws.com\"]}}}}"),
				responseMsg:  "",
			},
			wantErr: false,
		},
		{
			name: "non default values for credential provider",
			fields: fields{
				client: http.Client{},
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns:        []string{"123456789.dkr.ecr.test-region.amazonaws.com"},
					DefaultCacheDuration: "24h",
				},
			},
			patchResponse: response{
				statusCode:   http.StatusNoContent,
				expectedBody: []byte("{\"kubernetes\":{\"credential-providers\":{\"ecr-credential-provider\":{\"cache-duration\":\"24h\",\"enabled\":true,\"image-patterns\":[\"123456789.dkr.ecr.test-region.amazonaws.com\"]}}}}"),
				responseMsg:  "",
			},
			wantErr: false,
		},
		{
			name: "multiple match images for credential provider",
			fields: fields{
				client: http.Client{},
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns:        []string{"123456789.dkr.ecr.test-region.amazonaws.com", "987654321.dkr.ecr.test-region.amazonaws.com"},
					DefaultCacheDuration: "24h",
				},
			},
			patchResponse: response{
				statusCode:   http.StatusNoContent,
				expectedBody: []byte("{\"kubernetes\":{\"credential-providers\":{\"ecr-credential-provider\":{\"cache-duration\":\"24h\",\"enabled\":true,\"image-patterns\":[\"123456789.dkr.ecr.test-region.amazonaws.com\",\"987654321.dkr.ecr.test-region.amazonaws.com\"]}}}}"),
				responseMsg:  "",
			},
			wantErr: false,
		},
		{
			name: "failed credential provider update",
			fields: fields{
				client: http.Client{},
				config: constants.CredentialProviderConfigOptions{
					ImagePatterns:        []string{constants.DefaultImagePattern},
					DefaultCacheDuration: constants.DefaultCacheDuration,
				},
			},
			patchResponse: response{
				statusCode:   http.StatusNotFound,
				expectedBody: []byte("{\"kubernetes\":{\"credential-providers\":{\"ecr-credential-provider\":{\"cache-duration\":\"30m\",\"enabled\":true,\"image-patterns\":[\"*.dkr.ecr.*.amazonaws.com\"]}}}}"),
				responseMsg:  "",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodPatch {
						validatePatchRequest(w, r, t, tt.patchResponse)
					} else {
						t.Errorf("Recieved unexected request %v", r.Method)
					}
				}),
			)

			b := &bottleRocket{
				client:  tt.fields.client,
				baseURL: svr.URL + "/",
				config:  tt.fields.config,
			}
			if err := b.UpdateCredentialProvider(""); (err != nil) != tt.wantErr {
				t.Errorf("UpdateCredentialProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func validatePatchRequest(w http.ResponseWriter, r *http.Request, t *testing.T, patchResponse response) {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		t.Errorf("failed to read response")
	}
	if !bytes.Equal(data, patchResponse.expectedBody) {
		t.Errorf("Patch message expcted %v got %v", patchResponse.expectedBody, data)
	}
	w.WriteHeader(patchResponse.statusCode)
	fmt.Fprintf(w, patchResponse.responseMsg)
}

func Test_bottleRocket_Initialize(t *testing.T) {
	type args struct {
		config constants.CredentialProviderConfigOptions
	}
	tests := []struct {
		name    string
		baseUrl string
		args    args
	}{
		{
			name:    "simple initialization",
			baseUrl: "http://localhost/",
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
			b := &bottleRocket{}
			b.Initialize(tt.args.config)
			assert.Equal(t, tt.baseUrl, b.baseURL)
			assert.Equal(t, tt.args.config, b.config)
			assert.NotNil(t, b.client)
		})
	}
}
