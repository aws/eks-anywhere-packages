package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecrpublic"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type SDKClients struct {
	ecrClient              *ecrClient
	ecrPublicClient        *ecrPublicClient
	stsClient              *stsClient
	ecrClientRelease       *ecrClient
	ecrPublicClientRelease *ecrPublicClient
	stsClientRelease       *stsClient
}

// GetSDKClients is used to handle the creation of different SDK clients.
func GetSDKClients() (*SDKClients, error) {
	clients := &SDKClients{}
	var err error

	// STS Connection with us-west-2 region
	// This override ensures we don't create source clients for staging or prod accounts
	os.Setenv("AWS_PROFILE", "default")
	conf, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(ecrRegion))
	if err != nil {
		return nil, fmt.Errorf("loading default AWS config: %w", err)
	}
	stsclient := sts.NewFromConfig(conf)
	clients.stsClient, err = NewStsClient(stsclient, true)
	if err != nil {
		return nil, fmt.Errorf("creating SDK connection to STS %s", err)
	}

	ecrClient := ecr.NewFromConfig(conf)
	clients.ecrClient, err = NewECRClient(ecrClient, true)
	if err != nil {
		return nil, fmt.Errorf("creating SDK connection to ECR %s", err)
	}

	clients.stsClientRelease = clients.stsClient
	clients.ecrClientRelease = clients.ecrClient

	// ECR Public Connection with us-east-1 region
	conf, err = config.LoadDefaultConfig(context.TODO(), config.WithRegion(ecrPublicRegion))
	if err != nil {
		return nil, fmt.Errorf("loading default AWS config: %w", err)
	}
	client := ecrpublic.NewFromConfig(conf)
	clients.ecrPublicClient, err = NewECRPublicClient(client, true)
	if err != nil {
		return nil, fmt.Errorf("creating default public ECR client: %w", err)
	}

	return clients, nil
}

func (c *SDKClients) GetProfileSDKConnection(service, profile, region string) (*SDKClients, error) {
	if service == "" || profile == "" {
		return nil, fmt.Errorf("empty service or profile passed to GetProfileSDKConnection")
	}
	confWithProfile, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithSharedConfigProfile(profile))
	if err != nil {
		return nil, fmt.Errorf("creating public AWS client config: %w", err)
	}

	switch service {
	case "ecrpublic":
		clientWithProfile := ecrpublic.NewFromConfig(confWithProfile)
		c.ecrPublicClientRelease, err = NewECRPublicClient(clientWithProfile, true)
		if err != nil {
			return nil, fmt.Errorf("creating SDK connection to ECR Public using another profile %s", err)
		}
		return c, nil
	case "ecr":
		clientWithProfile := ecr.NewFromConfig(confWithProfile)
		c.ecrClientRelease, err = NewECRClient(clientWithProfile, true)
		if err != nil {
			return nil, fmt.Errorf("creating SDK connection to ECR using another profile %s", err)
		}
		return c, nil
	case "sts":
		clientWithProfile := sts.NewFromConfig(confWithProfile)
		c.stsClientRelease, err = NewStsClient(clientWithProfile, true)
		if err != nil {
			return nil, fmt.Errorf("creating SDK connection to STS using another profile %s", err)
		}
		return c, nil
	}
	return nil, fmt.Errorf("gave service not supported by GetProfileSDKConnection(), consider adding it to the switch case")
}
