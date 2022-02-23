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
					APIVersion: "packages.eks.amazonaws.com/api",
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
										Name: "v0.0",
										Tag:  "sha256:da25f5fdff88c259bb2ce7c0f1e9edddaf102dc4fb9cf5159ad6b902b5194e66",
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
				Name:       "cert-manager",
				Registry:   "public.ecr.aws/f5b7k4z5",
				Repository: "cert-manager",
				Versions:   []Tag{},
			},
			wantErr: false,
			wantBundle: &api.BundlePackage{
				Name: "cert-manager",
				Source: api.BundlePackageSource{
					Registry:   "public.ecr.aws/f5b7k4z5",
					Repository: "cert-manager",
					Versions:   []api.SourceVersion{},
				},
			},
		},
		{
			testname: "Test 1 tag",
			testproject: Project{
				Name:       "cert-manager",
				Registry:   "public.ecr.aws/f5b7k4z5",
				Repository: "cert-manager",
				Versions:   []Tag{{Name: "v1.0"}},
			},
			wantErr: false,
			wantBundle: &api.BundlePackage{
				Name: "cert-manager",
				Source: api.BundlePackageSource{
					Registry:   "public.ecr.aws/f5b7k4z5",
					Repository: "cert-manager",
					Versions: []api.SourceVersion{
						{
							Name: "v1.0",
							Tag:  "sha256:950385098ceafc5fb510b1d203fa18165047598a09292a8f040b7812a882c256",
						},
					},
				},
			},
		},
		{
			testname: "Test 2 tag",
			testproject: Project{
				Name:       "cert-manager",
				Registry:   "public.ecr.aws/f5b7k4z5",
				Repository: "cert-manager",
				Versions: []Tag{
					{Name: "v1.0"},
					{Name: "v1.1"},
				},
			},
			wantErr: false,
			wantBundle: &api.BundlePackage{
				Name: "cert-manager",
				Source: api.BundlePackageSource{
					Registry:   "public.ecr.aws/f5b7k4z5",
					Repository: "cert-manager",
					Versions: []api.SourceVersion{
						{
							Name: "v1.0",
							Tag:  "sha256:950385098ceafc5fb510b1d203fa18165047598a09292a8f040b7812a882c256",
						},
						{
							Name: "v1.1",
							Tag:  "sha256:ce0e42ab4f362252fd7706d4abe017a2c52743c4b3c6e56a9554c912ffddebcd",
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
