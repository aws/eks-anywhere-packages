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

package webhook

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/bundle"
)

type activeBundleValidator struct {
	Client      client.Client
	Config      *rest.Config
	decoder     *admission.Decoder
	kubeVersion string
}

func InitActiveBundleValidator(mgr ctrl.Manager) error {
	kubeVersion, err := getKubeServerVersion(mgr.GetConfig())
	if err != nil {
		return err
	}
	mgr.GetWebhookServer().
		Register("/validate-packages-eks-amazonaws-com-v1alpha1-packagebundlecontroller",
			&webhook.Admission{Handler: &activeBundleValidator{
				Client:      mgr.GetClient(),
				Config:      mgr.GetConfig(),
				kubeVersion: kubeVersion,
			}})
	return nil
}

func (v *activeBundleValidator) Handle(ctx context.Context, req admission.Request) admission.Response {

	pbc := &v1alpha1.PackageBundleController{}
	err := v.decoder.Decode(req, pbc)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError,
			fmt.Errorf("decoding request: %w", err))
	}

	bundles := &v1alpha1.PackageBundleList{}
	err = v.Client.List(ctx, bundles, &client.ListOptions{Namespace: v1alpha1.PackageNamespace})
	if err != nil {
		return admission.Errored(http.StatusInternalServerError,
			fmt.Errorf("listing package bundles: %w", err))
	}

	resp, err := v.handleInner(ctx, pbc, bundles)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return *resp
}

func (v *activeBundleValidator) handleInner(_ context.Context, pbc *v1alpha1.PackageBundleController, bundles *v1alpha1.PackageBundleList) (
	*admission.Response, error) {

	var found bool
	var theBundle v1alpha1.PackageBundle
	for _, b := range bundles.Items {
		if b.Name == pbc.Spec.ActiveBundle {
			theBundle = b
			found = true
			break
		}
	}

	if !found {
		msg := fmt.Sprintf("package bundle not theBundle with name: %q", pbc.Spec.ActiveBundle)
		resp := &admission.Response{
			AdmissionResponse: admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Status:  metav1.StatusFailure,
					Code:    http.StatusNotFound,
					Message: msg,
					Reason:  metav1.StatusReasonNotFound,
				},
			},
		}
		return resp, nil
	}

	matches, err := theBundle.KubeVersionMatches(v.kubeVersion)
	if err != nil {
		return nil, fmt.Errorf("listing package bundles: %w", err)
	}

	if !matches {
		msg := fmt.Sprintf("kuberneetes version %s does not match %s", v.kubeVersion, pbc.Spec.ActiveBundle)
		resp := &admission.Response{
			AdmissionResponse: admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Status:  metav1.StatusFailure,
					Code:    http.StatusBadRequest,
					Message: msg,
					Reason:  metav1.StatusReasonInvalid,
				},
			},
		}
		return resp, nil
	}

	resp := &admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{
			Allowed: true,
		},
	}
	return resp, nil
}

func getKubeServerVersion(cfg *rest.Config) (serverVersion string, err error) {
	disco, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return "", fmt.Errorf("creating discovery client: %s", err)
	}
	info, err := disco.ServerVersion()
	if err != nil {
		return "", fmt.Errorf("getting server version: %s", err)
	}
	return bundle.FormatKubeServerVersion(*info), nil
}

var _ admission.DecoderInjector = (*activeBundleValidator)(nil)

// InjectDecoder injects the decoder.
func (v *activeBundleValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
