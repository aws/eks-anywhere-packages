package main

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecrpublic"
	"github.com/aws/aws-sdk-go-v2/service/ecrpublic/types"
	"github.com/aws/aws-sdk-go/aws"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

type ecrClient struct {
	*ecrpublic.Client
}

// NewECRClient Creates a new ECR Client Public client
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

// Describe returns a list of ECR describe results, with Pagination from DescribeImages SDK request
func (c *ecrClient) Describe(describeInput *ecrpublic.DescribeImagesInput) ([]types.ImageDetail, error) {
	var images []types.ImageDetail
	resp, err := c.DescribeImages(context.TODO(), describeInput)
	if err != nil {
		return nil, fmt.Errorf("error: Unable to complete DescribeImagesRequest to ECR public. %s", err)
	}
	images = append(images, resp.ImageDetails...)
	if resp.NextToken != nil {
		next := describeInput
		next.NextToken = resp.NextToken
		nextdetails, _ := c.Describe(next)
		images = append(images, nextdetails...)
	}
	return images, nil
}

// GetShaForInputs returns a list of an images version/sha for given inputs to lookup
func (c *ecrClient) GetShaForInputs(project Project) ([]api.SourceVersion, error) {
	sourceVersion := []api.SourceVersion{}
	for _, tag := range project.Versions {
		if !strings.Contains(tag.Name, "latest") {
			var imagelookup []types.ImageIdentifier
			imagelookup = append(imagelookup, types.ImageIdentifier{ImageTag: &tag.Name})
			ImageDetails, err := c.Describe(&ecrpublic.DescribeImagesInput{
				RepositoryName: aws.String(project.Repository),
				ImageIds:       imagelookup,
			})
			if err != nil {
				return nil, fmt.Errorf("error: Unable to complete DescribeImagesRequest to ECR public. %s", err)
			}
			for _, images := range ImageDetails {
				if *images.ImageManifestMediaType != "application/vnd.oci.image.manifest.v1+json" || len(images.ImageTags) == 0 {
					continue
				}
				if len(images.ImageTags) == 1 {
					v := &api.SourceVersion{Name: tag.Name, Digest: *images.ImageDigest}
					sourceVersion = append(sourceVersion, *v)
					continue
				}
			}
		}
		//
		if tag.Name == "latest" {
			ImageDetails, err := c.Describe(&ecrpublic.DescribeImagesInput{
				RepositoryName: aws.String(project.Repository),
			})
			if err != nil {
				return nil, fmt.Errorf("error: Unable to complete DescribeImagesRequest to ECR public. %s", err)
			}
			sha, err := getLastestImageSha(ImageDetails)
			if err != nil {
				return nil, err
			}
			sourceVersion = append(sourceVersion, *sha)
			continue
		}
		//
		if strings.Contains(tag.Name, "-latest") {
			regex := regexp.MustCompile(`-latest`)
			splitVersion := regex.Split(tag.Name, -1) //extract out the version without the latest
			ImageDetails, err := c.Describe(&ecrpublic.DescribeImagesInput{
				RepositoryName: aws.String(project.Repository),
			})
			if err != nil {
				return nil, fmt.Errorf("error: Unable to complete DescribeImagesRequest to ECR public. %s", err)
			}
			filteredImageDetails := imageTagFilter(ImageDetails, splitVersion[0])
			sha, err := getLastestImageSha(filteredImageDetails)
			if err != nil {
				return nil, err
			}
			sourceVersion = append(sourceVersion, *sha)
			continue
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
	latest.ImagePushedAt = &time.Time{}
	for _, detail := range details {
		if len(details) < 1 || detail.ImagePushedAt == nil || detail.ImageDigest == nil || detail.ImageTags == nil || len(detail.ImageTags) == 0 || *detail.ImageManifestMediaType != "application/vnd.oci.image.manifest.v1+json" {
			continue
		}
		if detail.ImagePushedAt != nil && latest.ImagePushedAt.Before(*detail.ImagePushedAt) {
			latest = detail
		}
	}
	// Check if latest is equal to empty struct, and return error if that's the case.
	if reflect.DeepEqual(latest, types.ImageDetail{}) {
		return nil, fmt.Errorf("error no images found")
	}
	return &api.SourceVersion{Name: latest.ImageTags[0], Digest: *latest.ImageDigest}, nil
}

func imageTagFilter(details []types.ImageDetail, version string) []types.ImageDetail {
	var filteredDetails []types.ImageDetail
	for _, detail := range details {
		for _, tag := range detail.ImageTags {
			if strings.Contains(tag, version) {
				filteredDetails = append(filteredDetails, detail)
			}
		}
	}
	return filteredDetails
}

// stringInSlice checks to see if a string is in a slice
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// removeStringSlice removes a named string from a slice, without knowing it's index or it being ordered.
func removeStringSlice(l []string, item string) []string {
	for i, other := range l {
		if other == item {
			return append(l[:i], l[i+1:]...)
		}
	}
	return l
}

// Go 1.18 support for generics makes the above function more generic
// func remove[T comparable](l []T, item T) []T {
// 	for i, other := range l {
// 		if other == item {
// 			return append(l[:i], l[i+1:]...)
// 		}
// 	}
// 	return l
// }
