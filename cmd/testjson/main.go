package main

import (
	"encoding/json"
	"log"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

func main() {

	p := api.Package{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: api.PackageSpec{
			PackageName: "bar",
		},
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	err := encoder.Encode(NewDisplayPackage(p))
	if err != nil {
		log.Fatal(err)
	}

}

// DisplayPackage wraps Package to omit undesired members (like Status).
//
// This is necessary in part because of https://github.com/golang/go/issues/11939
// but also because we just don't want to generate a Status section when we're
// emitting templates for a user to modify.
type DisplayPackage struct {
	*api.Package
	Status *struct{} `json:"status,omitempty"`
}

func NewDisplayPackage(p api.Package) DisplayPackage {
	return DisplayPackage{Package: &p}
}
