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

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Package",type=string,JSONPath=`.spec.packageName`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="CurrentVersion",type=string,JSONPath=`.status.currentVersion`
// +kubebuilder:printcolumn:name="TargetVersion",type=string,JSONPath=`.status.targetVersion`
// +kubebuilder:printcolumn:name="Detail",type=string,JSONPath=`.status.detail`
// Package is the Schema for the package API
type Package struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PackageSpec   `json:"spec,omitempty"`
	Status PackageStatus `json:"status,omitempty"`
}

// PackageSpec defines the desired state of an package.
type PackageSpec struct {
	// PackageName is the name of the package as specified in the bundle.
	PackageName string `json:"packageName"`

	// PackageVersion is a human-friendly version name or sha256 checksum for the
	// package, as specified in the bundle.
	PackageVersion string `json:"packageVersion,omitempty"`

	// Config for the package
	Config string `json:"config,omitempty"`

	// TargetNamespace where package resources will be deployed.
	TargetNamespace string `json:"targetNamespace,omitempty"`
}

// +kubebuilder:validation:Enum=initializing;installing;installed;updating;uninstalling;unknown
type StateEnum string

const (
	StateInitializing StateEnum = "initializing"
	StateInstalling   StateEnum = "installing"
	StateInstalled    StateEnum = "installed"
	StateUpdating     StateEnum = "updating"
	StateUninstalling StateEnum = "uninstalling"
	StateUnknown      StateEnum = "unknown"
)

// PackageStatus defines the observed state of Package
type PackageStatus struct {
	// +kubebuilder:validation:Required
	// Source associated with the installation
	Source PackageOCISource `json:"source"`

	// +kubebuilder:validation:Required
	// Version currently installed
	CurrentVersion string `json:"currentVersion"`

	// +kubebuilder:validation:Required
	// Version to be installed
	TargetVersion string `json:"targetVersion,omitempty"`

	// State of the installation
	State StateEnum `json:"state,omitempty"`

	// Detail of the state
	Detail string `json:"detail,omitempty"`

	// UpgradesAvailable indicates upgraded versions in the bundle.
	UpgradesAvailable []PackageAvailableUpgrade `json:"upgradesAvailable,omitempty"`
}

// PackageAvailableUpgrade details the package's available upgrades' versions.
type PackageAvailableUpgrade struct {
	// +kubebuilder:validation:Required
	// Version is a human-friendly version name for the package upgrade.
	Version string `json:"version"`

	// +kubebuilder:validation:Required
	// Tag is a specific version number or sha256 checksum for the package
	// upgrade.
	Tag string `json:"tag"`
}

// +kubebuilder:object:root=true
// PackageList contains a list of Package
type PackageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Package `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Package{}, &PackageList{})
}
