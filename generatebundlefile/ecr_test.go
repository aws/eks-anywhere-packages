package main

import (
	"fmt"
	"os"
	"testing"
)

func TestPullHelmChart(t *testing.T) {
	tests := []struct {
		testName        string
		testHelmName    string
		testHelmVersion string
		helmLocation    string
		wantErr         bool
	}{
		{
			testName:        "Test empty name",
			testHelmName:    "",
			testHelmVersion: "0.1.1+9b09ef845d5d38d5201b96e32ae0be0ce2402b78",
			helmLocation:    "",
			wantErr:         true,
		},
		{
			testName:        "Test empty version",
			testHelmName:    "646717423341.dkr.ecr.us-west-2.amazonaws.com/hello-eks-anywhere",
			testHelmVersion: "",
			helmLocation:    "",
			wantErr:         true,
		},
		{
			testName:        "Test valid helm",
			testHelmName:    "646717423341.dkr.ecr.us-west-2.amazonaws.com/hello-eks-anywhere",
			testHelmVersion: "0.1.1+9b09ef845d5d38d5201b96e32ae0be0ce2402b78",
			helmLocation:    fmt.Sprintf("%s/Library/Caches/helm/repository/hello-eks-anywhere-0.1.1+9b09ef845d5d38d5201b96e32ae0be0ce2402b78.tgz", os.Getenv("HOME")),
			wantErr:         false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.testName, func(tt *testing.T) {
			got, err := PullHelmChart(tc.testHelmName, tc.testHelmVersion)
			if (err != nil) != tc.wantErr {
				t.Fatalf("PullHelmChart() error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.helmLocation {
				t.Fatalf("PullHelmChart() = %#v\n\n\n, want %#v", got, tc.helmLocation)
			}
		})
	}
}

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
			testName:     "Test valid name w/ prefix",
			testHelmName: "646717423341.dkr.ecr.us-west-2.amazonaws.com/aws-containers/hello-eks-anywhere",
			chartName:    "aws-containers/hello-eks-anywhere",
			helmName:     "hello-eks-anywhere",
			wantErr:      false,
		},
		{
			testName:     "Test invalid name w/ multiple prefixes",
			testHelmName: "646717423341.dkr.ecr.us-west-2.amazonaws.com/test/aws-containers/hello-eks-anywhere",
			chartName:    "",
			helmName:     "",
			wantErr:      true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.testName, func(tt *testing.T) {
			got, got2, err := splitECRName(tc.testHelmName)
			if (err != nil) != tc.wantErr {
				t.Fatalf("splitECRName() error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.chartName || got2 != tc.helmName {
				t.Fatalf("splitECRName() = %#v\n\n\n, want %#v %#v", got, tc.chartName, tc.helmName)
			}
		})
	}
}

//	err = UnTarHelmChart(chartPath, chartName, dest)

func TestUnTarHelmChart(t *testing.T) {
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
			dest:          fmt.Sprintf("%s/go/src/github.com/modelrocket/eks-anywhere-packages/generatebundlefile/hello-eks-anywhere", os.Getenv("HOME")),
			wantErr:       true,
		},
		{
			testName:      "Test empty ChartName",
			testChartPath: fmt.Sprintf("%s/Library/Caches/helm/repository/hello-eks-anywhere-0.1.1+9b09ef845d5d38d5201b96e32ae0be0ce2402b78.tgz", os.Getenv("HOME")),
			testChartName: "",
			dest:          fmt.Sprintf("%s/go/src/github.com/modelrocket/eks-anywhere-packages/generatebundlefile/hello-eks-anywhere", os.Getenv("HOME")),
			wantErr:       true,
		},
		{
			testName:      "Test empty dest",
			testChartPath: fmt.Sprintf("%s/Library/Caches/helm/repository/hello-eks-anywhere-0.1.1+9b09ef845d5d38d5201b96e32ae0be0ce2402b78.tgz", os.Getenv("HOME")),
			testChartName: "hello-eks-anywhere",
			dest:          "",
			wantErr:       true,
		},
		{
			testName:      "Test valid values",
			testChartPath: fmt.Sprintf("%s/Library/Caches/helm/repository/hello-eks-anywhere-0.1.1+9b09ef845d5d38d5201b96e32ae0be0ce2402b78.tgz", os.Getenv("HOME")),
			testChartName: "hello-eks-anywhere",
			dest:          fmt.Sprintf("%s/go/src/github.com/modelrocket/eks-anywhere-packages/generatebundlefile/hello-eks-anywhere", os.Getenv("HOME")),
			wantErr:       false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.testName, func(tt *testing.T) {
			err := UnTarHelmChart(tc.testChartPath, tc.testChartName, tc.dest)
			if (err != nil) != tc.wantErr {
				t.Fatalf("UnTarHelmChart() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestShaExistsInRepository(t *testing.T) {
	tests := []struct {
		testName       string
		testRepository string
		testVersion    string
		checkPass      bool
		wantErr        bool
	}{
		{
			testName:       "Test empty Repository",
			testRepository: "",
			testVersion:    "sha256:0526725a65691944e831add6b247b25a93b8eeb1033dddadeaa089e95b021172",
			wantErr:        true,
		},
		{
			testName:       "Test empty Version",
			testRepository: "hello-eks-anywhere",
			testVersion:    "",
			wantErr:        true,
		},
		{
			testName:       "Test valid Repository and Version",
			testRepository: "hello-eks-anywhere",
			testVersion:    "sha256:0526725a65691944e831add6b247b25a93b8eeb1033dddadeaa089e95b021172",
			checkPass:      true,
			wantErr:        false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.testName, func(tt *testing.T) {
			ecrPublicClient, err := NewECRPublicClient(true)
			if err != nil {
				t.Fatalf("ecrPublicClient() did not work, %v", err)
			}
			got, err := ecrPublicClient.shaExistsInRepository(tc.testRepository, tc.testVersion)
			if (err != nil) != tc.wantErr {
				t.Fatalf("shaExistsInRepository() error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.checkPass {
				t.Fatalf("shaExistsInRepository() = %#v\n\n\n, want %#v", got, tc.checkPass)
			}
		})
	}
}

func TestTagFromSha(t *testing.T) {
	tests := []struct {
		testName       string
		testRepository string
		testDigest     string
		checkVersion   string
		wantErr        bool
	}{
		{
			testName:       "Test empty Repository",
			testRepository: "",
			testDigest:     "sha256:0526725a65691944e831add6b247b25a93b8eeb1033dddadeaa089e95b021172",
			wantErr:        true,
		},
		{
			testName:       "Test empty Version",
			testRepository: "hello-eks-anywhere",
			testDigest:     "",
			wantErr:        true,
		},
		{
			testName:       "Test valid Repository and Version",
			testRepository: "hello-eks-anywhere",
			testDigest:     "sha256:0526725a65691944e831add6b247b25a93b8eeb1033dddadeaa089e95b021172",
			checkVersion:   "v0.1.1-baa4ef89fe91d65d3501336d95b680f8ae2ea660",
			wantErr:        false,
		},
	}
	//images.Digest, err = ecrClient.tagFromSha(images.Repository, images.Digest)
	for _, tc := range tests {
		t.Run(tc.testName, func(tt *testing.T) {
			ecrClient, err := NewECRClient(true)
			if err != nil {
				t.Fatalf("ecrClient() did not work, %v", err)
			}
			got, err := ecrClient.tagFromSha(tc.testRepository, tc.testDigest)
			if (err != nil) != tc.wantErr {
				t.Fatalf("tagFromSha() error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.checkVersion {
				t.Fatalf("tagFromSha() = %#v\n\n\n, want %#v", got, tc.checkVersion)
			}
		})
	}
}
