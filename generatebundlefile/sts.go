package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type stsClient struct {
	*sts.Client
	AccountID string
}

func NewStsClient(stsclient *sts.Client, account bool) (*stsClient, error) {
	stsClient := &stsClient{Client: stsclient}
	if account {
		stslookup, err := stsClient.Client.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
		if err != nil {
			return nil, err
		}
		if *stslookup.Account != "" {
			stsClient.AccountID = *stslookup.Account
			return stsClient, nil
		}
		return nil, fmt.Errorf("Empty Account ID from stslookup call")
	}
	return stsClient, nil
}
