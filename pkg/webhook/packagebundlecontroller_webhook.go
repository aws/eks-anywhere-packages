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
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/authenticator"
)

type activeBundleValidator struct {
	Client  client.Client
	Config  *rest.Config
	decoder *admission.Decoder
	tcc     authenticator.TargetClusterClient
}

func InitPackageBundleControllerValidator(mgr ctrl.Manager) error {
	tcc := authenticator.NewTargetClusterClient(mgr.GetLogger(), mgr.GetConfig(), mgr.GetClient())
	mgr.GetWebhookServer().
		Register("/validate-packages-eks-amazonaws-com-v1alpha1-packagebundlecontroller",
			&webhook.Admission{Handler: &activeBundleValidator{
				Client: mgr.GetClient(),
				Config: mgr.GetConfig(),
				tcc:    tcc,
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

func (v *activeBundleValidator) handleInner(ctx context.Context, pbc *v1alpha1.PackageBundleController, bundles *v1alpha1.PackageBundleList) (
	*admission.Response, error) {

	if pbc.Spec.ActiveBundle == "" {
		resp := &admission.Response{
			AdmissionResponse: admissionv1.AdmissionResponse{
				Allowed: true,
			},
		}
		return resp, nil
	}

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
		reason := fmt.Sprintf("activeBundle %q not present on cluster", pbc.Spec.ActiveBundle)
		resp := &admission.Response{
			AdmissionResponse: admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Status:  metav1.StatusFailure,
					Code:    http.StatusBadRequest,
					Message: reason,
					Reason:  metav1.StatusReason(reason),
				},
			},
		}
		return resp, nil
	}
	info, err := v.tcc.GetServerVersion(ctx, pbc.Name)
	if err != nil {
		return nil, fmt.Errorf("getting server version for %s: %w", pbc.Name, err)
	}

	matches, err := theBundle.KubeVersionMatches(info)
	if err != nil {
		return nil, fmt.Errorf("listing package bundles: %w", err)
	}

	if !matches {
		reason := fmt.Sprintf("kuberneetes version v%s-%s does not match %s", info.Major, info.Minor, pbc.Spec.ActiveBundle)
		resp := &admission.Response{
			AdmissionResponse: admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Status:  metav1.StatusFailure,
					Code:    http.StatusBadRequest,
					Message: reason,
					Reason:  metav1.StatusReason(reason),
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

var _ admission.DecoderInjector = (*activeBundleValidator)(nil)

// InjectDecoder injects the decoder.
func (v *activeBundleValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
