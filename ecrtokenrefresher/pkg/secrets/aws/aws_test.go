package aws

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeECRClient struct {
	getToken func(ctx context.Context, input *ecr.GetAuthorizationTokenInput) (*ecr.GetAuthorizationTokenOutput, error)
}

func (f *fakeECRClient) GetAuthorizationTokenWithContext(ctx context.Context, input *ecr.GetAuthorizationTokenInput, _ ...request.Option) (*ecr.GetAuthorizationTokenOutput, error) {
	return f.getToken(ctx, input)
}

func useFakeECRClient(t *testing.T, client ecrClient, timeout time.Duration) {
	t.Helper()
	origClient, origTimeout := newECRClient, ecrRequestTimeout
	t.Cleanup(func() {
		newECRClient, ecrRequestTimeout = origClient, origTimeout
	})
	newECRClient = func() (ecrClient, error) { return client, nil }
	ecrRequestTimeout = timeout
	t.Setenv(envRegionName, regionDefault)
}

func TestGetCredentialsReturnsWhenECRIsUnreachable(t *testing.T) {
	// An airgapped cluster can't reach ECR: the call blocks until the context ends.
	useFakeECRClient(t, &fakeECRClient{
		getToken: func(ctx context.Context, _ *ecr.GetAuthorizationTokenInput) (*ecr.GetAuthorizationTokenOutput, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}, 50*time.Millisecond)

	start := time.Now()
	creds, err := (&AwsSecret{}).getCredentials()

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded), "want deadline exceeded, got %v", err)
	assert.Nil(t, creds)
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestGetCredentialsParsesToken(t *testing.T) {
	endpoint := "https://783794618700.dkr.ecr.us-west-2.amazonaws.com"
	token := base64.StdEncoding.EncodeToString([]byte("AWS:password"))
	var gotDeadline bool
	var gotRegistryIDs []string
	useFakeECRClient(t, &fakeECRClient{
		getToken: func(ctx context.Context, input *ecr.GetAuthorizationTokenInput) (*ecr.GetAuthorizationTokenOutput, error) {
			_, gotDeadline = ctx.Deadline()
			for _, id := range input.RegistryIds {
				gotRegistryIDs = append(gotRegistryIDs, *id)
			}
			return &ecr.GetAuthorizationTokenOutput{
				AuthorizationData: []*ecr.AuthorizationData{{
					AuthorizationToken: &token,
					ProxyEndpoint:      &endpoint,
				}},
			}, nil
		},
	}, time.Minute)

	creds, err := (&AwsSecret{}).getCredentials()

	require.NoError(t, err)
	assert.True(t, gotDeadline, "ECR request context has no deadline")
	assert.Equal(t, []string{defaultAccountID, devAccountID}, gotRegistryIDs)
	require.Len(t, creds, 1)
	assert.Equal(t, "AWS", creds[0].Username)
	assert.Equal(t, "password", creds[0].Password)
	assert.Equal(t, endpoint, creds[0].Registry)
}

func TestGetCredentialsReturnsECRError(t *testing.T) {
	useFakeECRClient(t, &fakeECRClient{
		getToken: func(context.Context, *ecr.GetAuthorizationTokenInput) (*ecr.GetAuthorizationTokenOutput, error) {
			return nil, errors.New("access denied")
		},
	}, time.Minute)

	_, err := (&AwsSecret{}).getCredentials()

	assert.ErrorContains(t, err, "access denied")
}
