package main

import (
	"context"
	"fmt"
	"log"
	"reflect"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecrpublic"
	"github.com/aws/aws-sdk-go-v2/service/ecrpublic/types"
	"github.com/aws/aws-sdk-go/aws"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

type ecrClient struct {
	*ecrpublic.Client
}

// Create ECR Client Public client
func NewECRClient() (*ecrClient, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"))
	if err != nil {
		log.Fatal(err)
	}
	ecrClient := &ecrClient{Client: ecrpublic.NewFromConfig(cfg)}
	if err != nil {
		return nil, err
	}
	return ecrClient, nil
}

// GetShaForTags returns a list of an images version/sha for given inputs to lookup
func (c *ecrClient) GetShaForTags(project Project) ([]api.SourceVersion, error) {
	resp, err := c.DescribeImages(context.TODO(), &ecrpublic.DescribeImagesInput{
		RepositoryName: aws.String(project.Name),
	})
	if err != nil {
		return nil, fmt.Errorf("error: Unable to complete DescribeImagesRequest to ECR public. %s", err)
	}
	// Lookup of the tags and corresponding sha's from the name matches from the input files.
	// EX input file has tag `v1.1.0-eks-a-4` this will check the ECR describe command to see if tag
	// `v1.1.0-eks-a-4` is present. If so it will return that chart's sha256 sum, if multiple tags of
	// `v1.1.0-eks-a-4` are present from the ecr lookup it will find whichever one was the most recently pushed and return that.
	sourceVersion := []api.SourceVersion{}
	for _, images := range resp.ImageDetails {
		// Check for Helm Chart Media Type
		// Helm = application/vnd.oci.image.manifest.v1+json
		if *images.ImageManifestMediaType != "application/vnd.oci.image.manifest.v1+json" {
			continue
		}
		for _, tag := range images.ImageTags {
			// if it is nil, we don't do to check for matches
			if tag == "" || images.ImageDigest == nil {
				continue
			}
			matchingTags := project.Matches(tag)
			switch {
			case len(matchingTags) == 1:
				v := &api.SourceVersion{Name: matchingTags[0], Digest: *images.ImageDigest}
				sourceVersion = append(sourceVersion, *v)
			// If we get more than 1 tag match, we lookup whichever was the most recent push ex: two images are labeled with v1.1 we used the most recent one.
			case len(matchingTags) > 1:
				sha, err := getLastestImageSha(resp.ImageDetails)
				if err != nil {
					return nil, err
				}
				sourceVersion = append(sourceVersion, *sha)
			}
		}
	}
	sourceVersion = removeDuplicates(sourceVersion)
	return sourceVersion, nil
}

// removeDuplicates removes any duplicates from Version list, useful for scenarios when
// multiple tags for an image are present, this would cause duplicates on the bundle CRD,
// so we remove the first one in this case since they are the same thing.
// EX sha1234 is tagged 1.1 and 1.2 and sha5678 is tagged 1.2 this would result in a double match of 1.2 so we run this.
func removeDuplicates(s []api.SourceVersion) []api.SourceVersion {
	k := make(map[string]bool)
	l := []api.SourceVersion{}
	for _, i := range s {
		if _, j := k[i.Name]; !j {
			k[i.Name] = true
			l = append(l, api.SourceVersion{Name: i.Name, Digest: i.Digest})
		}
	}
	return l
}

// getLastestImageSha returns the tag and SHA-256 checksum of the latest pushed image
// It returns an error if an empty slice is provided, or if the latest image's
// name or SHA-256 checksum are nil.
// getLastestImageSha Iterates list of images, to find latest pushed image and return tag/sha
func getLastestImageSha(details []types.ImageDetail) (*api.SourceVersion, error) {
	if len(details) == 0 {
		return nil, fmt.Errorf("no details provided")
	}
	var latest types.ImageDetail
	for _, detail := range details {
		if len(details) < 1 || detail.ImagePushedAt == nil || detail.ImageDigest == nil || detail.ImageTags == nil || len(detail.ImageTags) == 0 {
			continue
		}
		if detail.ImagePushedAt != nil && latest.ImagePushedAt.Before(*detail.ImagePushedAt) {
			latest = detail
		}
	}
	// Check if latest is equal to empty struct, and return error if that's the case.
	if reflect.ValueOf(latest).Interface() == reflect.ValueOf(types.ImageDetail{}).Interface() {
		return nil, fmt.Errorf("error no images found")
	}
	return &api.SourceVersion{Name: latest.ImageTags[0], Digest: *latest.ImageDigest}, nil
}
