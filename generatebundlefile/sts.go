package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type stsClient struct {
	*sts.Client
	AccountID string
}

func NewStsClient(account bool) (*stsClient, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-west-2"))
	if err != nil {
		return nil, fmt.Errorf("Creating AWS STS config %w", err)
	}
	stsClient := &stsClient{Client: sts.NewFromConfig(cfg)}
	if err != nil {
		return nil, err
	}
	if account {
		stslookup, err := stsClient.Client.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
		if err != nil {
			return nil, err
		}
		if *stslookup.Account != "" {
			stsClient.AccountID = *stslookup.Account
			return stsClient, nil
		}
	}
	return stsClient, nil
}
