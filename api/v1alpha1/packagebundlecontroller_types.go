// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultRegistry      = "public.ecr.aws/eks-anywhere"
	defaultImageRegistry = "783794618700.dkr.ecr.us-west-2.amazonaws.com"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=pbc,path=packagebundlecontrollers
// +kubebuilder:printcolumn:name="ActiveBundle",type=string,JSONPath=`.spec.activeBundle`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Detail",type=string,JSONPath=`.status.detail`
// PackageBundleController is the Schema for the packagebundlecontroller API.
type PackageBundleController struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PackageBundleControllerSpec   `json:"spec,omitempty"`
	Status PackageBundleControllerStatus `json:"status,omitempty"`
}

// PackageBundleControllerSpec defines the desired state of
// PackageBundleController.
type PackageBundleControllerSpec struct {
	// LogLevel controls the verbosity of logging in the controller.
	// +optional
	LogLevel *int32 `json:"logLevel,omitempty"`

	// +kubebuilder:default:="1d"
	// UpgradeCheckInterval is the time between upgrade checks.
	//
	// The format is that of time's ParseDuration.
	// +optional
	UpgradeCheckInterval metav1.Duration `json:"upgradeCheckInterval,omitempty"`

	// +kubebuilder:default:="1h"
	// UpgradeCheckShortInterval time between upgrade checks if there is a problem.
	//
	// The format is that of time's ParseDuration.
	// +optional
	UpgradeCheckShortInterval metav1.Duration `json:"upgradeCheckShortInterval,omitempty"`

	// ActiveBundle is name of the bundle from which packages should be sourced.
	// +optional
	ActiveBundle string `json:"activeBundle"`

	// PrivateRegistry is the registry being used for all images, charts and bundles
	// +optional
	PrivateRegistry string `json:"privateRegistry"`

	// +kubebuilder:default:="public.ecr.aws/eks-anywhere"
	// DefaultRegistry for pulling helm charts and the bundle
	// +optional
	DefaultRegistry string `json:"defaultRegistry"`

	// +kubebuilder:default:="783794618700.dkr.ecr.us-west-2.amazonaws.com"
	// DefaultImageRegistry for pulling images
	// +optional
	DefaultImageRegistry string `json:"defaultImageRegistry"`

	// +kubebuilder:default:="eks-anywhere-packages-bundles"
	// Repository portion of an OCI address to the bundle
	// +optional
	BundleRepository string `json:"bundleRepository"`
}

// +kubebuilder:validation:Enum=ignored;active;disconnected
type BundleControllerStateEnum string

const (
	BundleControllerStateIgnored          BundleControllerStateEnum = "ignored"
	BundleControllerStateActive           BundleControllerStateEnum = "active"
	BundleControllerStateUpgradeAvailable BundleControllerStateEnum = "upgrade available"
	BundleControllerStateDisconnected     BundleControllerStateEnum = "disconnected"
)

// PackageBundleControllerStatus defines the observed state of
// PackageBundleController.
type PackageBundleControllerStatus struct {
	// State of the bundle controller.
	State BundleControllerStateEnum `json:"state,omitempty"`

	// Detail of the state.
	Detail string `json:"detail,omitempty"`
}

// +kubebuilder:object:root=true
// PackageBundleControllerList contains a list of PackageBundleController.
type PackageBundleControllerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PackageBundleController `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PackageBundleController{}, &PackageBundleControllerList{})
}
