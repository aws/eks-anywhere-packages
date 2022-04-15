package main

import (
	"reflect"
	"testing"

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
				},
				Spec: api.PackageBundleSpec{
					KubeVersion: "1.21",
					Packages: []api.BundlePackage{
						{
							Name: "sample-package",
							Source: api.BundlePackageSource{
								Registry:   "sample-Registry",
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
				t.Fatalf("GetClusterConfig() = %#v, want %#v", got, tc.wantBundle)
			}
		})
	}
}

func TestNewPackageFromInput(t *testing.T) {
	tests := []struct {
		testname    string
		testproject Project
		wantErr     bool
		wantBundle  *api.BundlePackage
	}{
		{
			testname: "Test no tags",
			testproject: Project{
				Name:       "hello-eks-anywhere",
				Registry:   "public.ecr.aws/f5b7k4z5",
				Repository: "aws-containers/hello-eks-anywhere",
				Versions:   []Tag{},
			},
			wantErr: true,
		},
		{
			testname: "Test named tag",
			testproject: Project{
				Name:       "hello-eks-anywhere",
				Registry:   "public.ecr.aws/f5b7k4z5",
				Repository: "aws-containers/hello-eks-anywhere",
				Versions: []Tag{
					{Name: "0.1.0_c4e25cb42e9bb88d2b8c2abfbde9f10ade39b214"},
				},
			},
			wantErr: false,
			wantBundle: &api.BundlePackage{
				Name: "hello-eks-anywhere",
				Source: api.BundlePackageSource{
					Registry:   "public.ecr.aws/f5b7k4z5",
					Repository: "aws-containers/hello-eks-anywhere",
					Versions: []api.SourceVersion{
						{
							Name:   "0.1.0_c4e25cb42e9bb88d2b8c2abfbde9f10ade39b214",
							Digest: "sha256:d5467083c4d175e7e9bba823e95570d28fff86a2fbccb03f5ec3093db6f039be",
						},
					},
				},
			},
		},
		{
			testname: "Test 'latest' tag",
			testproject: Project{
				Name:       "hello-eks-anywhere",
				Registry:   "public.ecr.aws/f5b7k4z5",
				Repository: "aws-containers/hello-eks-anywhere",
				Versions: []Tag{
					{Name: "latest"},
				},
			},
			wantErr: false,
			wantBundle: &api.BundlePackage{
				Name: "hello-eks-anywhere",
				Source: api.BundlePackageSource{
					Registry:   "public.ecr.aws/f5b7k4z5",
					Repository: "aws-containers/hello-eks-anywhere",
					Versions: []api.SourceVersion{
						{
							Name:   "0.1.0_c4e25cb42e9bb88d2b8c2abfbde9f10ade39b214",
							Digest: "sha256:d5467083c4d175e7e9bba823e95570d28fff86a2fbccb03f5ec3093db6f039be",
						},
					},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.testname, func(tt *testing.T) {
			got, err := tc.testproject.NewPackageFromInput()
			if (err != nil) != tc.wantErr {
				t.Fatalf("NewPackageFromInput() error = %v, wantErr %v", err, tc.wantErr)
			}
			if !reflect.DeepEqual(got, tc.wantBundle) {
				t.Fatalf("NewPackageFromInput() = %#v\n\n\n, want %#v", got, tc.wantBundle)
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
					APIVersion: SchemeBuilder.GroupVersion.String(),
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
					APIVersion: SchemeBuilder.GroupVersion.String(),
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
				t.Fatalf("IfSignature() error = %v", err)
			}
			if got != tc.wantBool {
				t.Fatalf("IfSignature() = %#v\n\n\n, want %#v", got, tc.wantBool)
			}
		})
	}
}

func TestAddSignature(t *testing.T) {
	tests := []struct {
		testname          string
		testbundle        *api.PackageBundle
		wantPackageBundle *api.PackageBundle
		signature         string
		wantErr           bool
	}{
		{
			testname: "Test empty signature",
			testbundle: &api.PackageBundle{
				TypeMeta: metav1.TypeMeta{
					Kind:       api.PackageBundleKind,
					APIVersion: SchemeBuilder.GroupVersion.String(),
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
			testname: "Test add signature",
			testbundle: &api.PackageBundle{
				TypeMeta: metav1.TypeMeta{
					Kind:       api.PackageBundleKind,
					APIVersion: SchemeBuilder.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "1.20",
					Namespace: "eksa-packages",
				},
			},
			signature: "signature-123",
			wantPackageBundle: &api.PackageBundle{
				TypeMeta: metav1.TypeMeta{
					Kind:       api.PackageBundleKind,
					APIVersion: SchemeBuilder.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "1.20",
					Namespace: "eksa-packages",
					Annotations: map[string]string{
						"eksa.aws.com/signature": "signature-123",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.testname, func(tt *testing.T) {
			got, err := AddSignature(tc.testbundle, tc.signature)
			if (err != nil) != tc.wantErr {
				t.Fatalf("AddSignature() error = %v, wantErr %v", err, tc.wantErr)
			}
			if !reflect.DeepEqual(got, tc.wantPackageBundle) {
				t.Fatalf("AddSignature() = %#v\n\n\n, want %#v", got, tc.wantPackageBundle)
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
					APIVersion: SchemeBuilder.GroupVersion.String(),
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
					APIVersion: SchemeBuilder.GroupVersion.String(),
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
					APIVersion: SchemeBuilder.GroupVersion.String(),
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
				t.Fatalf("CheckSignature() error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.wantBool {
				t.Fatalf("CheckSignature() = %#v\n\n\n, want %#v", got, tc.wantBool)
			}
		})
	}
}
