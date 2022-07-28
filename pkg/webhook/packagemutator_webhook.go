package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/eks-anywhere-packages/pkg/types"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"
	"strconv"
	"strings"

	"github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/bundle"
)

type packageMutator struct {
	Client       client.Client
	BundleClient bundle.Client
	decoder      *admission.Decoder
}

var apilog = ctrl.Log.WithName("webhook")

func InitPackageMutator(mgr ctrl.Manager) error {
	mgr.GetWebhookServer().
		Register("/mutate-packages-eks-amazonaws-com-v1alpha1-package",
			&webhook.Admission{Handler: &packageMutator{
				Client:       mgr.GetClient(),
				BundleClient: bundle.NewPackageBundleClient(mgr.GetClient()),
			}})
	return nil
}

func (m *packageMutator) Handle(ctx context.Context, request admission.Request) admission.Response {
	apilog.Info("Package Mutator Called!!")
	p := &v1alpha1.Package{}
	err := m.decoder.Decode(request, p)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError,
			fmt.Errorf("decoding request: %w", err))
	}

	activeBundle, err := m.BundleClient.GetActiveBundle(ctx)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("getting PackageBundle: %v", err))
	}
	packageInBundle, err := activeBundle.GetPackageFromBundle(p.Spec.PackageName)
	jsonSchema, err := packageInBundle.GetJsonSchema()

	setDefaults(p, jsonSchema)
	newPackage, err := json.Marshal(p)
	return admission.PatchResponseFromRaw(request.Object.Raw, newPackage)
}

func setDefaults(p *v1alpha1.Package, jsonSchema []byte) {
	currentConfigs, _ := p.GetValues()
	// Update currentConfigs with Json Schema
	schemaObj := &types.Schema{}
	_ = json.Unmarshal(jsonSchema, schemaObj)

	for key, val := range schemaObj.Properties {
		keySegments := strings.Split(key, ".")
		updateDefault(currentConfigs, keySegments, 0, val.Default)
	}

	updatedConfigs, _ := yaml.Marshal(currentConfigs)
	p.Spec.Config = string(updatedConfigs)
}

func updateDefault(values map[string]interface{}, keySegments []string, index int, val string) {
	if index >= len(keySegments) {
		return
	}

	key := keySegments[index]
	if index == len(keySegments)-1 {
		if _, ok := values[key]; !ok {
			if bVal, err := strconv.ParseBool(val); err == nil {
				values[key] = bVal
			} else {
				values[key] = val
			}
		}
		return
	}
	if _, ok := values[key].(string); ok {
		return
	}
	if _, ok := values[key]; !ok {
		values[key] = map[string]interface{}{}
	}

	updateDefault(values[key].(map[string]interface{}), keySegments, index+1, val)
}

// InjectDecoder injects the decoder.
func (m *packageMutator) InjectDecoder(d *admission.Decoder) error {
	m.decoder = d
	return nil
}
