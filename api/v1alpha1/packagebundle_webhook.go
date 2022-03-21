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
	"encoding/base64"
	"errors"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/aws/eks-anywhere-packages/pkg/signature"
)

const (
	PublicKeyEnvVar = "EKSA_PUBLIC_KEY"
)

// apilog is for logging in this package.
var apilog = ctrl.Log.WithName("api")

func (r *PackageBundle) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-packages-eks-amazonaws-com-v1alpha1-packagebundle,mutating=false,failurePolicy=fail,sideEffects=None,groups=packages.eks.amazonaws.com,resources=packagebundles,verbs=create;update,versions=v1alpha1,name=vpackagebundle.kb.io,admissionReviewVersions=v1
var _ webhook.Validator = &PackageBundle{}

func (r *PackageBundle) ValidateCreate() error {
	return r.validate()
}

func (r *PackageBundle) ValidateUpdate(old runtime.Object) error {
	return r.validate()
}

func (r *PackageBundle) ValidateDelete() error {
	return nil
}

func (r *PackageBundle) validate() error {
	//TODO Turn this into fields in a new CRD used for signature validation configuration
	keyOverride := os.Getenv(PublicKeyEnvVar)
	domain := signature.EksaDomain
	if keyOverride != "" {
		domain = signature.Domain{Name: signature.DomainName, Pubkey: keyOverride}
	}
	valid, digest, yml, err := signature.ValidateSignature(r, domain)
	if err != nil {
		return err
	}
	if !valid {
		apilog.Info("Invalid signature", "Error", err, "Digest", base64.StdEncoding.EncodeToString(digest[:]), "Manifest", string(yml))
		return errors.New("The signature is invalid for the configured public key: " + domain.Pubkey)
	}
	return nil
}
