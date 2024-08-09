package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type stsClient struct {
	stsClientInterface
	AccountID string
}

type stsClientInterface interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

func NewStsClient(client stsClientInterface, account bool) (*stsClient, error) {
	stsClient := &stsClient{stsClientInterface: client}
	if account {
		stslookup, err := stsClient.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
		if err != nil {
			return nil, err
		}
		if *stslookup.Account != "" {
			stsClient.AccountID = *stslookup.Account
			return stsClient, nil
		}
		return nil, fmt.Errorf("empty Account ID from stslookup call")
	}
	return stsClient, nil
}
