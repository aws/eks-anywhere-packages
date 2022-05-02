package main

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

// +kubebuilder:object:generate=false
// Same as Bundle except stripped down for generation of yaml file during generate bundleconfig
type BundleGenerate struct {
	// TypeMeta   metav1.TypeMeta `json:",inline"`
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec api.PackageBundleSpec `json:"spec,omitempty"`
}

type Time struct {
	time.Time `protobuf:"-"`
}

type ObjectMetaNoTimestamp struct {
	Name                       string                      `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	GenerateName               string                      `json:"generateName,omitempty" protobuf:"bytes,2,opt,name=generateName"`
	Namespace                  string                      `json:"namespace,omitempty" protobuf:"bytes,3,opt,name=namespace"`
	SelfLink                   string                      `json:"selfLink,omitempty" protobuf:"bytes,4,opt,name=selfLink"`
	UID                        types.UID                   `json:"uid,omitempty" protobuf:"bytes,5,opt,name=uid,casttype=k8s.io/kubernetes/pkg/types.UID"`
	ResourceVersion            string                      `json:"resourceVersion,omitempty" protobuf:"bytes,6,opt,name=resourceVersion"`
	Generation                 int64                       `json:"generation,omitempty" protobuf:"varint,7,opt,name=generation"`
	CreationTimestamp          interface{}                 `json:"creationTimestamp,omitempty"`
	DeletionTimestamp          *Time                       `json:"deletionTimestamp,omitempty" protobuf:"bytes,9,opt,name=deletionTimestamp"`
	DeletionGracePeriodSeconds *int64                      `json:"deletionGracePeriodSeconds,omitempty" protobuf:"varint,10,opt,name=deletionGracePeriodSeconds"`
	Labels                     map[string]string           `json:"labels,omitempty" protobuf:"bytes,11,rep,name=labels"`
	Annotations                map[string]string           `json:"annotations,omitempty" protobuf:"bytes,12,rep,name=annotations"`
	OwnerReferences            []metav1.OwnerReference     `json:"ownerReferences,omitempty" patchStrategy:"merge" patchMergeKey:"uid" protobuf:"bytes,13,rep,name=ownerReferences"`
	Finalizers                 []string                    `json:"finalizers,omitempty" patchStrategy:"merge" protobuf:"bytes,14,rep,name=finalizers"`
	ClusterName                string                      `json:"clusterName,omitempty" protobuf:"bytes,15,opt,name=clusterName"`
	ManagedFields              []metav1.ManagedFieldsEntry `json:"managedFields,omitempty" protobuf:"bytes,17,rep,name=managedFields"`
}

type PackageBundleNoTimestamp struct {
	metav1.TypeMeta       `json:",inline"`
	ObjectMetaNoTimestamp `json:"metadata,omitempty"`

	Spec   api.PackageBundleSpec   `json:"spec,omitempty"`
	Status api.PackageBundleStatus `json:"status,omitempty"`
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
	Images         []Image `json:"images,omitempty"`
	Configurations []Tag   `json:"configurations,omitempty"`
}

type Image struct {
	Repository string `json:"repository,omitempty"`
	Tag        string `json:"tag,omitempty"`
	Digest     string `json:"digest,omitempty"`
}

type Values struct {
	SourceRegistry string `json:"sourceRegistry,omitempty"`
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
