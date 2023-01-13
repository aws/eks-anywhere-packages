package main

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ecrpublic"
	ecrpublictypes "github.com/aws/aws-sdk-go-v2/service/ecrpublic/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

func TestNewBundleGenerate(t *testing.T) {
	tests := []struct {
		testname   string
		bundleName string
		wantBundle *api.PackageBundle
	}{
		{
			testname:   "TestNewBundleGenerate",
			bundleName: "bundlename",
			wantBundle: &api.PackageBundle{
				TypeMeta: metav1.TypeMeta{
					Kind:       "PackageBundle",
					APIVersion: "packages.eks.amazonaws.com/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bundlename",
					Namespace: "eksa-packages",
					Annotations: map[string]string{
						"eksa.aws.com/excludes": "LnNwZWMucGFja2FnZXNbXS5zb3VyY2UucmVnaXN0cnkKLnNwZWMucGFja2FnZXNbXS5zb3VyY2UucmVwb3NpdG9yeQo=",
					},
				},
				Spec: api.PackageBundleSpec{
					Packages: []api.BundlePackage{
						{
							Name: "sample-package",
							Source: api.BundlePackageSource{
								Repository: "sample-Repository",
								Versions: []api.SourceVersion{
									{
										Name:   "v0.0",
										Digest: "sha256:da25f5fdff88c259bb2ce7c0f1e9edddaf102dc4fb9cf5159ad6b902b5194e66",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.testname, func(tt *testing.T) {
			got := NewBundleGenerate(tc.bundleName)
			if !reflect.DeepEqual(got, tc.wantBundle) {
				tt.Fatalf("GetClusterConfig() = %#v, want %#v", got, tc.wantBundle)
			}
		})
	}
}

var testTagBundle string = "0.1.0_c4e25cb42e9bb88d2b8c2abfbde9f10ade39b214"
var testShaBundle string = "sha256:d5467083c4d175e7e9bba823e95570d28fff86a2fbccb03f5ec3093db6f039be"
var testImageMediaType string = "application/vnd.oci.image.manifest.v1+json"
var testRegistryId string = "public.ecr.aws/eks-anywhere"
var testRepositoryName string = "hello-eks-anywhere"

func TestNewPackageFromInput(t *testing.T) {
	client := newMockPublicRegistryClientBundle(nil)
	tests := []struct {
		client      *mockPublicRegistryClientBundle
		testname    string
		testproject Project
		wantErr     bool
		wantBundle  *api.BundlePackage
	}{
		{
			testname: "Test no tags",
			testproject: Project{
				Name:       "hello-eks-anywhere",
				Repository: "hello-eks-anywhere",
				Registry:   "public.ecr.aws/eks-anywhere",
				Versions:   []Tag{},
			},
			wantErr: true,
		},
		{
			testname: "Test named tag",
			testproject: Project{
				Name:       "hello-eks-anywhere",
				Repository: "hello-eks-anywhere",
				Registry:   "public.ecr.aws/eks-anywhere",
				Versions: []Tag{
					{Name: testTagBundle},
				},
			},
			wantErr: false,
			wantBundle: &api.BundlePackage{
				Name: "hello-eks-anywhere",
				Source: api.BundlePackageSource{
					Repository: "hello-eks-anywhere",
					Registry:   "public.ecr.aws/eks-anywhere",
					Versions: []api.SourceVersion{
						{
							Name:   testTagBundle,
							Digest: testShaBundle,
						},
					},
				},
			},
		},
		{
			testname: "Test 'latest' tag",
			testproject: Project{
				Name:       "hello-eks-anywhere",
				Repository: "hello-eks-anywhere",
				Registry:   "public.ecr.aws/eks-anywhere",
				Versions: []Tag{
					{Name: "latest"},
				},
			},
			wantErr: false,
			wantBundle: &api.BundlePackage{
				Name: "hello-eks-anywhere",
				Source: api.BundlePackageSource{
					Repository: "hello-eks-anywhere",
					Registry:   "public.ecr.aws/eks-anywhere",
					Versions: []api.SourceVersion{
						{
							Name:   testTagBundle,
							Digest: testShaBundle,
						},
					},
				},
			},
		},
		{
			testname: "Test '-latest' in the middle of tag",
			testproject: Project{
				Name:       "hello-eks-anywhere",
				Repository: "hello-eks-anywhere",
				Registry:   "public.ecr.aws/eks-anywhere",
				Versions: []Tag{
					{Name: "test-latest-helm"},
				},
			},
			wantErr: false,
			wantBundle: &api.BundlePackage{
				Name: "hello-eks-anywhere",
				Source: api.BundlePackageSource{
					Repository: "hello-eks-anywhere",
					Registry:   "public.ecr.aws/eks-anywhere",
					Versions: []api.SourceVersion{
						{
							Name:   "test-latest-helm",
							Digest: testShaBundle,
						},
					},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.testname, func(tt *testing.T) {
			clients := &SDKClients{
				ecrPublicClient: &ecrPublicClient{
					publicRegistryClient: client,
				},
			}
			got, err := clients.NewPackageFromInput(tc.testproject)
			if (err != nil) != tc.wantErr {
				tt.Fatalf("NewPackageFromInput() error = %v, wantErr %v got %v", err, tc.wantErr, got)
			}
			if !reflect.DeepEqual(got, tc.wantBundle) {
				tt.Fatalf("NewPackageFromInput() = %#v\n\n\n, want %#v", got, tc.wantBundle)
			}
		})
	}
}

func TestIfSignature(t *testing.T) {
	tests := []struct {
		testname   string
		testbundle *api.PackageBundle
		wantBool   bool
	}{
		{
			testname: "Test no annotations",
			testbundle: &api.PackageBundle{
				TypeMeta: metav1.TypeMeta{
					Kind:       api.PackageBundleKind,
					APIVersion: api.SchemeBuilder.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "1.20",
					Namespace: "eksa-packages",
				},
			},
			wantBool: false,
		},
		{
			testname: "Test with annotations",
			testbundle: &api.PackageBundle{
				TypeMeta: metav1.TypeMeta{
					Kind:       api.PackageBundleKind,
					APIVersion: api.SchemeBuilder.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "1.20",
					Namespace: "eksa-packages",
					Annotations: map[string]string{
						"eksa.aws.com/signature": "123",
					},
				},
			},
			wantBool: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.testname, func(tt *testing.T) {
			got, err := IfSignature(tc.testbundle)
			if err != nil {
				tt.Fatalf("IfSignature() error = %v", err)
			}
			if got != tc.wantBool {
				tt.Fatalf("IfSignature() = %#v\n\n\n, want %#v", got, tc.wantBool)
			}
		})
	}
}

func TestCheckSignature(t *testing.T) {
	tests := []struct {
		testname   string
		testbundle *api.PackageBundle
		signature  string
		wantBool   bool
		wantErr    bool
	}{
		{
			testname: "Test empty signature",
			testbundle: &api.PackageBundle{
				TypeMeta: metav1.TypeMeta{
					Kind:       api.PackageBundleKind,
					APIVersion: api.SchemeBuilder.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "1.20",
					Namespace: "eksa-packages",
				},
			},
			signature: "",
			wantErr:   true,
		},
		{
			testname:   "Test empty Bundle",
			testbundle: nil,
			signature:  "signature-123",
			wantErr:    true,
		},
		{
			testname: "Test same signature",
			testbundle: &api.PackageBundle{
				TypeMeta: metav1.TypeMeta{
					Kind:       api.PackageBundleKind,
					APIVersion: api.SchemeBuilder.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "1.20",
					Namespace: "eksa-packages",
					Annotations: map[string]string{
						"eksa.aws.com/signature": "signature-123",
					},
				},
			},
			signature: "signature-123",
			wantBool:  true,
			wantErr:   false,
		},
		{
			testname: "Test different signature",
			testbundle: &api.PackageBundle{
				TypeMeta: metav1.TypeMeta{
					Kind:       api.PackageBundleKind,
					APIVersion: api.SchemeBuilder.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "1.20",
					Namespace: "eksa-packages",
					Annotations: map[string]string{
						"eksa.aws.com/signature": "signature-456",
					},
				},
			},
			signature: "signature-123",
			wantBool:  false,
			wantErr:   true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.testname, func(tt *testing.T) {
			got, err := CheckSignature(tc.testbundle, tc.signature)
			if (err != nil) != tc.wantErr {
				tt.Fatalf("CheckSignature() error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.wantBool {
				tt.Fatalf("CheckSignature() = %#v\n\n\n, want %#v", got, tc.wantBool)
			}
		})
	}
}

type mockPublicRegistryClientBundle struct {
	err error
}

func newMockPublicRegistryClientBundle(err error) *mockPublicRegistryClientBundle {
	return &mockPublicRegistryClientBundle{
		err: err,
	}
}

func (r *mockPublicRegistryClientBundle) DescribeImages(ctx context.Context, params *ecrpublic.DescribeImagesInput, optFns ...func(*ecrpublic.Options)) (*ecrpublic.DescribeImagesOutput, error) {
	if r.err != nil {
		return nil, r.err
	}
	testImagePushedAt := time.Now()
	return &ecrpublic.DescribeImagesOutput{
		ImageDetails: []ecrpublictypes.ImageDetail{
			{
				ImageDigest:            &testShaBundle,
				ImageTags:              []string{testTagBundle},
				ImageManifestMediaType: &testImageMediaType,
				ImagePushedAt:          &testImagePushedAt,
				RegistryId:             &testRegistryId,
				RepositoryName:         &testRepositoryName,
			},
		},
	}, nil
}

func (r *mockPublicRegistryClientBundle) DescribeRegistries(ctx context.Context, params *ecrpublic.DescribeRegistriesInput, optFns ...func(*ecrpublic.Options)) (*ecrpublic.DescribeRegistriesOutput, error) {
	panic("not implemented") // TODO: Implement
}

func (r *mockPublicRegistryClientBundle) GetAuthorizationToken(ctx context.Context, params *ecrpublic.GetAuthorizationTokenInput, optFns ...func(*ecrpublic.Options)) (*ecrpublic.GetAuthorizationTokenOutput, error) {
	panic("not implemented") // TODO: Implement
}
