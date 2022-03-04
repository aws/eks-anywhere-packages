package signature

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"path"
	"strings"
	"text/template"

	"github.com/itchyny/gojq"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const (
	PublicKey               = "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAETP8OUc6rTZHJs98X1aJfDIO0BXihHnSBDJhacdxZwk9RVzq28OIxQSVXXhD5ATEqWcNSWLnCG/GrZY9W2NfoMw=="
	SignatureAnnotation     = "signature"
	ExcludesAnnotation      = "excludes"
	FullSignatureAnnotation = "eksa.aws.com/signature"
)

var EksaDomain = Domain{Name: "eksa.aws.com", Pubkey: PublicKey}

type GojqParams struct {
	Excludes []string
	Domain   Domain
}

var (
	AlwaysExcluded = []string{".status", ".metadata.creationTimestamp", ".metadata.generation", ".metadata.managedFields", ".metadata.uid", ".metadata.resourceVersion"}
	GojqTemplate   = template.Must(template.New("gojq_query").Funcs(
		template.FuncMap{
			"StringsJoin": strings.Join,
			"Escape": func(in string) string {
				return strings.ReplaceAll(in, ".", "\\\\.")
			},
		}).Parse(`
del({{ StringsJoin .Excludes ", "}}) | .metadata.annotations |= with_entries(select(.key | test("^{{ Escape .Domain.Name }}/(?:includes|excludes)$") ))
`))
)

type Manifest = metav1.ObjectMetaAccessor

func filter(in []string) []string {
	filtered := in[:0]
	for _, s := range in {
		if s != "" {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

func decodeSelectors(selectorsB64Encoded string) (selectors []string, err error) {
	decoded, err := base64.StdEncoding.DecodeString(selectorsB64Encoded)
	if err != nil {
		return selectors, err
	}
	selectors = filter(strings.Split(string(decoded), "\n"))
	for _, arg := range selectors {
		parsed, err := gojq.Parse(arg)
		if err != nil {
			return nil, err
		}
		if parsed.Term == nil || parsed.Term.Type != gojq.TermTypeIndex {
			return nil, errors.New("Invalid selector(s) provided")
		}
	}
	return selectors, err
}

func GetMetadataInformation(manifest Manifest, domain Domain) (signature string, excludes []string, err error) {
	meta := manifest.GetObjectMeta()
	annotations := meta.GetAnnotations()
	signature, sigExists := annotations[path.Join(domain.Name, SignatureAnnotation)]
	excludesB64, excludesExists := annotations[path.Join(domain.Name, ExcludesAnnotation)]
	if !sigExists {
		err = errors.New("Missing signature")
		return signature, excludes, err
	}

	if excludesExists {
		excludes, err = decodeSelectors(excludesB64)
		if err != nil {
			return signature, excludes, err
		}
	}
	return signature, excludes, err
}

func GetDigest(manifest Manifest, domain Domain) (digest [32]byte, yml []byte, err error) {
	var query *gojq.Query
	_, excludes, err := GetMetadataInformation(manifest, domain)
	if err != nil {
		return [32]byte{}, nil, err
	}

	renderedQuery := &bytes.Buffer{}
	err = GojqTemplate.Execute(renderedQuery, GojqParams{
		append(excludes, AlwaysExcluded...),
		domain,
	})
	if err != nil {
		return [32]byte{}, nil, err
	}
	query, err = gojq.Parse(renderedQuery.String())
	if err != nil {
		return [32]byte{}, nil, err
	}
	// gojq requires running on raw types, marshal and unmarshall to allow it.
	asjson, _ := json.Marshal(manifest)
	var unmarshalled interface{}
	_ = json.Unmarshal(asjson, &unmarshalled)
	jsonIt := query.Run(unmarshalled)
	filtered, remaining := jsonIt.Next()
	if remaining {
		second, rem := jsonIt.Next()
		if second != nil && !rem {
			return [32]byte{}, nil, errors.New("Multiple result from the query should never happen")
		}
	}

	yml, err = yaml.Marshal(filtered)
	if err != nil {
		return [32]byte{}, nil, errors.New("Manifest could not be marshaled to yaml")
	}
	digest = sha256.Sum256(yml)
	return digest, yml, err
}

func ValidateSignature(manifest Manifest, domain Domain) (valid bool, digest [32]byte, yml []byte, err error) {
	// Shell equivalent
	// Obtain the yaml
	//    < bundle.yaml yq -y --indentless-lists --sort-keys   \
	//    '.spec | .bundles = (.bundles | [.[] | walk( if type == "object" then with_entries(select(.value != "" and .value != null and .value != [])) else . end)])' \
	//        | sha256sum
	//
	// Sign with kms
	//AWS_REGION=us-east-2 aws kms sign --key-id alias/demo --message \
	// file://<(< ~/dev/proj/eks-anywhere-packages/api/v1alpha1/testdata/addons_v1alpha_addonbundle_signature_good.yaml \
	//   yq --indentless-lists -y -S 'del(.spec.bundles[].chart.registry, .spec.bundles[].chart.repository, .metadata.annotations) |  \
	//      walk( if type == "object" then with_entries(select(.value != "" and .value != null and .value != [])) else . end)'  \
	//          --message-type RAW --signing-algorithm ECDSA_SHA_256

	metaSig, _, err := GetMetadataInformation(manifest, domain)
	if err != nil {
		return false, [32]byte{}, yml, err
	}
	digest, yml, err = GetDigest(manifest, domain)
	if err != nil {
		return false, [32]byte{}, yml, err
	}

	sig, err := base64.StdEncoding.DecodeString(metaSig)
	if err != nil {
		return false, digest, yml, errors.New("Signature in metadata isn't base64 encoded")
	}
	pubdecoded, err := base64.StdEncoding.DecodeString(domain.Pubkey)
	if err != nil {
		return false, digest, yml, errors.New("Unable to decode the public key (not base 64)")
	}
	pubparsed, err := x509.ParsePKIXPublicKey(pubdecoded)
	if err != nil {
		return false, digest, yml, errors.New("Unable parse the public key (not PKIX)")
	}
	pubkey := pubparsed.(*ecdsa.PublicKey)

	return ecdsa.VerifyASN1(pubkey, digest[:], sig), digest, yml, nil
}
