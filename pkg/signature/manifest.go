package signature

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"text/template"

	"github.com/itchyny/gojq"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const (
	PublicKey               = "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEnP0Yo+ZxzPUEfohcG3bbJ8987UT4f0tj+XVBjS/s35wkfjrxTKrVZQpz3ta3zi5ZlgXzd7a20B1U1Py/TtPsxw=="
	DomainName              = "eksa.aws.com"
	SignatureAnnotation     = "signature"
	ExcludesAnnotation      = "excludes"
	FullSignatureAnnotation = "eksa.aws.com/signature"
)

var EksaDomain = Domain{Name: DomainName, Pubkey: PublicKey}

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

const (
	ExcludesSep = "\n"
	DefaultHash = crypto.SHA256
)

var Encoding = base64.StdEncoding

func decodeSelectors(selectorsB64Encoded string) (selectors []string, err error) {
	decoded, err := Encoding.DecodeString(selectorsB64Encoded)
	if err != nil {
		return selectors, err
	}
	selectors = filter(strings.Split(string(decoded), ExcludesSep))
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

func GetDigest(manifest Manifest, domain Domain) (digest []byte, yml []byte, err error) {
	var query *gojq.Query
	_, excludes, err := GetMetadataInformation(manifest, domain)
	if err != nil {
		return nil, nil, err
	}

	renderedQuery := &bytes.Buffer{}
	err = GojqTemplate.Execute(renderedQuery, GojqParams{
		append(excludes, AlwaysExcluded...),
		domain,
	})
	if err != nil {
		return nil, nil, err
	}
	query, err = gojq.Parse(renderedQuery.String())
	if err != nil {
		return nil, nil, err
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
			return nil, nil, errors.New("Multiple result from the query should never happen")
		}
	}

	yml, err = yaml.Marshal(filtered)
	if err != nil {
		return nil, nil, errors.New("Manifest could not be marshaled to yaml")
	}

	h := DefaultHash.New()
	_, err = h.Write(yml)
	if err != nil {
		return nil, nil, fmt.Errorf("calculating checksum: %w", err)
	}

	return h.Sum(nil), yml, err
}

//See ./testdata/sign_file.sh for a shell script implementation.
//This here differs in that it normalizes quoting while the shell script doesnt (yet).
func ValidateSignature(manifest Manifest, domain Domain) (valid bool, digest []byte, yml []byte, err error) {
	metaSig, _, err := GetMetadataInformation(manifest, domain)
	if err != nil {
		return false, nil, yml, err
	}
	digest, yml, err = GetDigest(manifest, domain)
	if err != nil {
		return false, nil, yml, err
	}

	sig, err := Encoding.DecodeString(metaSig)
	if err != nil {
		return false, nil, nil, errors.New("Signature in metadata isn't base64 encoded")
	}
	pubdecoded, err := Encoding.DecodeString(domain.Pubkey)
	if err != nil {
		return false, nil, nil, errors.New("Unable to decode the public key (not base 64)")
	}
	pubparsed, err := x509.ParsePKIXPublicKey(pubdecoded)
	if err != nil {
		return false, nil, yml, errors.New("Unable parse the public key (not PKIX)")
	}
	pubkey := pubparsed.(*ecdsa.PublicKey)

	return ecdsa.VerifyASN1(pubkey, digest, sig), digest, yml, nil
}
