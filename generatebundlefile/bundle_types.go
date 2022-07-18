package main

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// +kubebuilder:object:generate=false
// Same as Bundle except stripped down for generation of yaml file during generate bundleconfig
type BundleGenerate struct {
	// TypeMeta   metav1.TypeMeta `json:",inline"`
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec api.PackageBundleSpec `json:"spec,omitempty"`
}

// SigningPackageBundle removes fields that shouldn't be included when signing.
type SigningPackageBundle struct {
	*api.PackageBundle

	// The fields below are to be modified or removed before signing.

	// SigningObjectMeta removes fields that shouldn't be included when signing.
	*SigningObjectMeta `json:"metadata,omitempty"`

	// Status isn't relevant to a digital signature.
	Status interface{} `json:"status,omitempty"`
}

// newSigningPackageBundle is api.PackageBundle using SigningObjectMeta instead of metav1.ObjectMeta
func newSigningPackageBundle(bundle *api.PackageBundle) *SigningPackageBundle {
	return &SigningPackageBundle{
		PackageBundle:     bundle,
		SigningObjectMeta: newSigningObjectMeta(&bundle.ObjectMeta),
		Status:            nil,
	}
}

// SigningObjectMeta removes fields that shouldn't be included when signing.
type SigningObjectMeta struct {
	*metav1.ObjectMeta

	// The fields below are to be removed before signing.

	// CreationTimestamp isn't relevant to a digital signature.
	CreationTimestamp interface{} `json:"creationTimestamp,omitempty"`
}

// newSigningObjectMeta is metav1.ObjectMeta without the CreationTimestamp since it gets added the yaml as null otherwise.
func newSigningObjectMeta(meta *metav1.ObjectMeta) *SigningObjectMeta {
	return &SigningObjectMeta{
		ObjectMeta:        meta,
		CreationTimestamp: nil,
	}
}

// Types for input file format
// +kubebuilder:object:root=true
// Input is the schema for the Input file
type Input struct {
	Packages          []Org  `json:"packages,omitempty"`
	Name              string `json:"name,omitempty"`
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`
}

// Projects object containing the input file github org and repo locations
type Org struct {
	Org      string    `json:"org,omitempty"`
	Projects []Project `json:"projects,omitempty"`
}

// Repos is the object containing the project within the github org, and the release tag
type Project struct {
	Name       string `json:"name,omitempty"`
	Registry   string `json:"registry,omitempty"`
	Repository string `json:"repository,omitempty"`
	Versions   []Tag  `json:"versions,omitempty"`
}

// Tag is the release tag
type Tag struct {
	Name string `json:"name,omitempty"`
}

type Requires struct {
	Kind              string `json:"kind,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RequiresSpec `json:"spec,omitempty"`
}

type RequiresSpec struct {
	Images         []Image         `json:"images,omitempty"`
	Configurations []Configuration `json:"configurations,omitempty"`
}

type Configuration struct {
	Name     string              `json:"name,omitempty"`
	Required bool                `json:"required,omitempty"`
	Default  *apiextensions.JSON `json:"default,omitempty"`
}

type Image struct {
	Repository string `json:"repository,omitempty"`
	Tag        string `json:"tag,omitempty"`
	Digest     string `json:"digest,omitempty"`
}

type Values struct {
	SourceRegistry string `json:"sourceRegistry,omitempty"`
}

type DockerAuth struct {
	Auths map[string]DockerAuthRegistry `json:"auths,omitempty"`
}

type DockerAuthRegistry struct {
	Auth string `json:"auth"`
}

type DockerAuthFile struct {
	Authfile string `json:"authfile"`
}

// Matches returns a list of inputs which align with ECR tags that exist
func (project Project) Matches(tag string) []string {
	matchlist := []string{}
	for _, version := range project.Versions {
		if version.Name == tag {
			matchlist = append(matchlist, version.Name)
		}
	}
	return matchlist
}
