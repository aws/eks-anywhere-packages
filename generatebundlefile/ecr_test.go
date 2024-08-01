package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

func TestSplitECRName(t *testing.T) {
	tests := []struct {
		testName     string
		testHelmName string
		chartName    string
		helmName     string
		wantErr      bool
	}{
		{
			testName:     "Test empty name",
			testHelmName: "",
			chartName:    "",
			helmName:     "",
			wantErr:      true,
		},
		{
			testName:     "Test valid name no prefix",
			testHelmName: "646717423341.dkr.ecr.us-west-2.amazonaws.com/hello-eks-anywhere",
			chartName:    "hello-eks-anywhere",
			helmName:     "hello-eks-anywhere",
			wantErr:      false,
		},
		{
			testName:     "Test valid name w/ 1 prefix",
			testHelmName: "646717423341.dkr.ecr.us-west-2.amazonaws.com/hello-eks-anywhere",
			chartName:    "hello-eks-anywhere",
			helmName:     "hello-eks-anywhere",
			wantErr:      false,
		},
		{
			testName:     "Test valid name w/ 2 prefixes",
			testHelmName: "646717423341.dkr.ecr.us-west-2.amazonaws.com/test/hello-eks-anywhere",
			chartName:    "test/hello-eks-anywhere",
			helmName:     "hello-eks-anywhere",
			wantErr:      false,
		},
		{
			testName:     "Test valid name w/ 3 prefixes",
			testHelmName: "646717423341.dkr.ecr.us-west-2.amazonaws.com/test/testing/hello-eks-anywhere",
			chartName:    "test/testing/hello-eks-anywhere",
			helmName:     "hello-eks-anywhere",
			wantErr:      false,
		},
		{
			testName:     "Test valid name w/ 4 prefixes",
			testHelmName: "646717423341.dkr.ecr.us-west-2.amazonaws.com/test/tested/testing/hello-eks-anywhere",
			chartName:    "test/tested/testing/hello-eks-anywhere",
			helmName:     "hello-eks-anywhere",
			wantErr:      false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.testName, func(tt *testing.T) {
			got, got2, err := splitECRName(tc.testHelmName)
			if (err != nil) != tc.wantErr {
				tt.Fatalf("splitECRName() error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.chartName || got2 != tc.helmName {
				tt.Fatalf("splitECRName() = %#v\n\n\n, want %#v %#v", got, tc.chartName, tc.helmName)
			}
		})
	}
}

func TestUnTarHelmChart(t *testing.T) {
	// Construct a Valid temp Helm Chart and Targz it.
	var tarGZ string = "test.tgz"
	err := os.Mkdir("hello-eks-anywhere", 0o750)
	if err != nil {
		t.Fatal("Error creating test dir:", err)
	}
	defer os.RemoveAll("hello-eks-anywhere")
	f, _ := os.Create("hello-eks-anywhere/Chart.yaml")
	content := []byte("apiVersion: v2\nversion: 0.1.0\nappVersion: 0.1.0\nname: hello-eks-anywhere\n")
	err = os.WriteFile("hello-eks-anywhere/Chart.yaml", content, 0o644)
	if err != nil {
		t.Fatal("Error creating test files:", err)
	}
	defer f.Close()
	out, err := os.Create(tarGZ)
	if err != nil {
		t.Fatal("Error creating test .tar:", err)
	}
	defer out.Close()
	files := []string{f.Name()}
	err = createArchive(files, out)
	if err != nil {
		t.Fatal("Error adding files to .tar:", err)
	}
	defer os.Remove(tarGZ)
	tests := []struct {
		testName      string
		testChartPath string
		testChartName string
		dest          string
		wantErr       bool
	}{
		{
			testName:      "Test empty ChartPath",
			testChartPath: "",
			testChartName: "hello-eks-anywhere",
			wantErr:       true,
		},
		{
			testName:      "Test empty ChartName",
			testChartPath: tarGZ,
			testChartName: "",
			wantErr:       true,
		},
		{
			testName:      "Test valid values",
			testChartPath: tarGZ,
			testChartName: "hello-eks-anywhere",
			wantErr:       false,
		},
	}
	for _, tc := range tests {
		tempDir := t.TempDir()
		t.Run(tc.testName, func(tt *testing.T) {
			err := UnTarHelmChart(tc.testChartPath, tc.testChartName, tempDir)
			if (err != nil) != tc.wantErr {
				tt.Fatalf("UnTarHelmChart() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestTagFromSha(t *testing.T) {
	client := newMockRegistryClient(nil)
	tests := []struct {
		client           *mockRegistryClient
		testName         string
		testRepository   string
		testDigest       string
		testSubstringTag string
		checkVersion     string
		wantErr          bool
	}{
		{
			testName:         "Test empty Repository",
			testRepository:   "",
			testDigest:       "sha256:0526725a65691944e831add6b247b25a93b8eeb1033dddadeaa089e95b021172",
			testSubstringTag: "",
			wantErr:          true,
		},
		{
			testName:         "Test empty Version",
			testRepository:   "hello-eks-anywhere",
			testDigest:       "",
			testSubstringTag: "",
			wantErr:          true,
		},
		{
			testName:         "Test valid Repository and Version",
			testRepository:   "hello-eks-anywhere",
			testDigest:       "sha256:0526725a65691944e831add6b247b25a93b8eeb1033dddadeaa089e95b021172",
			testSubstringTag: "v0.1.1-baa4ef89fe91d65d3501336d95b680f8ae2ea660",
			checkVersion:     "v0.1.1-baa4ef89fe91d65d3501336d95b680f8ae2ea660",
			wantErr:          false,
		},
		{
			testName:         "Test valid Repository and substring Version",
			testRepository:   "hello-eks-anywhere",
			testDigest:       "sha256:0526725a65691944e831add6b247b25a93b8eeb1033dddadeaa089e95b021172",
			testSubstringTag: "v0.1.1",
			checkVersion:     "v0.1.1-baa4ef89fe91d65d3501336d95b680f8ae2ea660",
			wantErr:          false,
		},
	}
	// images.Digest, err = ecrClient.tagFromSha(images.Repository, images.Digest)
	for _, tc := range tests {
		t.Run(tc.testName, func(tt *testing.T) {
			clients := &SDKClients{
				ecrClient: &ecrClient{
					registryClient: client,
				},
			}
			got, err := clients.ecrClient.tagFromSha(tc.testRepository, tc.testDigest, tc.checkVersion)
			if (err != nil) != tc.wantErr {
				tt.Fatalf("tagFromSha() error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.checkVersion {
				tt.Fatalf("tagFromSha() = %#v\n\n\n, want %#v", got, tc.checkVersion)
			}
		})
	}
}

// ECR

type mockRegistryClient struct {
	err error
}

func newMockRegistryClient(err error) *mockRegistryClient {
	return &mockRegistryClient{
		err: err,
	}
}

var (
	testTag1 string = "v0.1.1-baa4ef89fe91d65d3501336d95b680f8ae2ea660"
	testTag2 string = "v0.1.1-baa4ef89fe91d65d3501336d95b680f8ae2ea661"
)

func (r *mockRegistryClient) DescribeImages(ctx context.Context, params *ecr.DescribeImagesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error) {
	if r.err != nil {
		return nil, r.err
	}
	return &ecr.DescribeImagesOutput{
		ImageDetails: []ecrtypes.ImageDetail{
			{
				ImageTags: []string{testTag1, testTag2},
			},
		},
	}, nil
}

func (r *mockRegistryClient) GetAuthorizationToken(ctx context.Context, params *ecr.GetAuthorizationTokenInput, optFns ...func(*ecr.Options)) (*ecr.GetAuthorizationTokenOutput, error) {
	panic("not implemented") // TODO: Implement
}

func createArchive(files []string, buf io.Writer) error {
	// Create new Writers for gzip and tar
	// These writers are chained. Writing to the tar writer will
	// write to the gzip writer which in turn will write to
	// the "buf" writer
	gw := gzip.NewWriter(buf)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Iterate over files and add them to the tar archive
	for _, file := range files {
		err := addToArchive(tw, file)
		if err != nil {
			return err
		}
	}

	return nil
}

func addToArchive(tw *tar.Writer, filename string) error {
	// Open the file which will be written into the archive
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Get FileInfo about our file providing file size, mode, etc.
	info, err := file.Stat()
	if err != nil {
		return err
	}

	// Create a tar Header from the FileInfo data
	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return err
	}

	// Use full path as name (FileInfoHeader only takes the basename)
	header.Name = filename

	// Write file header to the tar archive
	err = tw.WriteHeader(header)
	if err != nil {
		return err
	}

	// Copy file content to tar archive
	_, err = io.Copy(tw, file)
	if err != nil {
		return err
	}

	return nil
}
