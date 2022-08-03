package v1alpha1_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

func TestPackageBundle_Find(t *testing.T) {
	var err error
	givenBundle := func(versions []api.SourceVersion) api.PackageBundle {
		return api.PackageBundle{
			Spec: api.PackageBundleSpec{
				Packages: []api.BundlePackage{
					{
						Name: "hello-eks-anywhere",
						Source: api.BundlePackageSource{
							Registry:   "public.ecr.aws/l0g8r8j6",
							Repository: "hello-eks-anywhere",
							Versions:   versions,
						},
					},
				},
			},
		}
	}
	sut := givenBundle(
		[]api.SourceVersion{
			{
				Name:   "0.1.0",
				Digest: "sha256:eaa07ae1c06ffb563fe3c16cdb317f7ac31c8f829d5f1f32442f0e5ab982c3e7",
			},
		},
	)

	expected := api.PackageOCISource{
		Registry:   "public.ecr.aws/l0g8r8j6",
		Repository: "hello-eks-anywhere",
		Digest:     "sha256:eaa07ae1c06ffb563fe3c16cdb317f7ac31c8f829d5f1f32442f0e5ab982c3e7",
		Version:    "0.1.0",
	}

	actual, err := sut.FindSource("hello-eks-anywhere", "0.1.0")
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)

	actual, err = sut.FindSource("hello-eks-anywhere", "sha256:eaa07ae1c06ffb563fe3c16cdb317f7ac31c8f829d5f1f32442f0e5ab982c3e7")
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)

	expectedErr := "package not found in bundle (fake bundle): Bogus @ bar"
	sut.ObjectMeta.Name = "fake bundle"
	_, err = sut.FindSource("Bogus", "bar")
	assert.EqualError(t, err, expectedErr)

	t.Run("Get latest version returns the first item", func(t *testing.T) {
		latest := givenBundle(
			[]api.SourceVersion{
				{
					Name:   "0.1.1",
					Digest: "sha256:deadbeef",
				},
				{
					Name:   "0.1.0",
					Digest: "sha256:eaa07ae1c06ffb563fe3c16cdb317f7ac31c8f829d5f1f32442f0e5ab982c3e7",
				},
			},
		)
		expected := api.PackageOCISource{
			Registry:   "public.ecr.aws/l0g8r8j6",
			Repository: "hello-eks-anywhere",
			Digest:     "sha256:deadbeef",
			Version:    "0.1.1",
		}
		actual, err = latest.FindSource("hello-eks-anywhere", api.Latest)
		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	})

	t.Run("Get latest version returns the first item even if the name describes a later version", func(t *testing.T) {
		latest := givenBundle(
			[]api.SourceVersion{
				{
					Name:   "0.1.0",
					Digest: "sha256:eaa07ae1c06ffb563fe3c16cdb317f7ac31c8f829d5f1f32442f0e5ab982c3e7",
				},
				{
					Name:   "0.1.1",
					Digest: "sha256:deadbeef",
				},
			},
		)
		expected := api.PackageOCISource{
			Registry:   "public.ecr.aws/l0g8r8j6",
			Repository: "hello-eks-anywhere",
			Digest:     "sha256:eaa07ae1c06ffb563fe3c16cdb317f7ac31c8f829d5f1f32442f0e5ab982c3e7",
			Version:    "0.1.0",
		}
		actual, err = latest.FindSource("hello-eks-anywhere", api.Latest)
		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	})
}

func TestGetMajorMinorFromString(t *testing.T) {

	t.Run("Parse from default Kubernetes version name", func(t *testing.T) {

		targetVersion := "v1-21-1"
		major, minor := api.GetMajorMinorFromString(targetVersion)

		assert.Equal(t, 1, major)
		assert.Equal(t, 21, minor)
	})

	t.Run("Parse from Kubernetes version name without patch number", func(
		t *testing.T) {

		targetVersion := "v1-21"
		major, minor := api.GetMajorMinorFromString(targetVersion)

		assert.Equal(t, 1, major)
		assert.Equal(t, 21, minor)
	})

	t.Run("Parse from Kubernetes version name without v perfix", func(
		t *testing.T) {

		targetVersion := "1-21-1"
		major, minor := api.GetMajorMinorFromString(targetVersion)

		assert.Equal(t, 1, major)
		assert.Equal(t, 21, minor)
	})

	t.Run("Parse from empty Kubernetes version name", func(t *testing.T) {

		targetVersion := ""
		major, minor := api.GetMajorMinorFromString(targetVersion)

		assert.Equal(t, 0, major)
		assert.Equal(t, 0, minor)
	})
}

func TestKubeVersionMatches(t *testing.T) {

	bundle := api.PackageBundle{ObjectMeta: metav1.ObjectMeta{
		Name: "v1-21-1001"}}

	t.Run("Kubernetes version matches", func(t *testing.T) {

		targetVersion := "v1-21-1"

		result, err := bundle.KubeVersionMatches(targetVersion)

		assert.True(t, result)
		assert.Nil(t, err)
	})

	t.Run("Kubernetes major version doesn't match", func(t *testing.T) {

		targetVersion := "v2-21-1"

		result, err := bundle.KubeVersionMatches(targetVersion)

		assert.False(t, result)
		assert.Nil(t, err)
	})

	t.Run("Kubernetes minor version doesn't match", func(t *testing.T) {

		targetVersion := "v1-22-1"

		result, err := bundle.KubeVersionMatches(targetVersion)

		assert.False(t, result)
		assert.Nil(t, err)
	})

	t.Run("bogus major", func(t *testing.T) {
		bundle := api.PackageBundle{ObjectMeta: metav1.ObjectMeta{
			Name: "vx-21-1001"}}
		targetVersion := "v1-21"

		result, err := bundle.KubeVersionMatches(targetVersion)

		assert.False(t, result)
		assert.EqualError(t, err, "inavlid major number <vx-21-1001>")
	})

	t.Run("bogus minor", func(t *testing.T) {
		bundle := api.PackageBundle{ObjectMeta: metav1.ObjectMeta{
			Name: "v1-x-1001"}}
		targetVersion := "v1-21"

		result, err := bundle.KubeVersionMatches(targetVersion)

		assert.False(t, result)
		assert.EqualError(t, err, "inavlid minor number <v1-x-1001>")
	})

	t.Run("bogus build", func(t *testing.T) {
		bundle := api.PackageBundle{ObjectMeta: metav1.ObjectMeta{
			Name: "v1-21-x"}}
		targetVersion := "v1-22"

		result, err := bundle.KubeVersionMatches(targetVersion)

		assert.False(t, result)
		assert.EqualError(t, err, "inavlid build number <v1-21-x>")
	})
}

func TestPackageMatches(t *testing.T) {
	orig := api.BundlePackageSource{
		Registry:   "registry",
		Repository: "repository",
		Versions: []api.SourceVersion{
			{Name: "v1", Digest: "sha256:deadbeef"},
			{Name: "v2", Digest: "sha256:cafebabe"},
		},
	}

	t.Run("package matches", func(t *testing.T) {
		other := api.BundlePackageSource{
			Registry:   "registry",
			Repository: "repository",
			Versions: []api.SourceVersion{
				{Name: "v1", Digest: "sha256:deadbeef"},
				{Name: "v2", Digest: "sha256:cafebabe"},
			},
		}
		result := orig.PackageMatches(other)
		if !result {
			t.Errorf("expected <%t> got <%t>", true, result)
		}
	})

	t.Run("package registries must match", func(t *testing.T) {
		other := api.BundlePackageSource{
			Registry:   "registry2",
			Repository: "repository",
			Versions: []api.SourceVersion{
				{Name: "v1", Digest: "sha256:deadbeef"},
				{Name: "v2", Digest: "sha256:cafebabe"},
			},
		}
		result := orig.PackageMatches(other)
		if result {
			t.Errorf("expected <%t> got <%t>", false, result)
		}
	})

	t.Run("package repositories must match", func(t *testing.T) {
		other := api.BundlePackageSource{
			Registry:   "registry",
			Repository: "repository2",
			Versions: []api.SourceVersion{
				{Name: "v1", Digest: "sha256:deadbeef"},
				{Name: "v2", Digest: "sha256:cafebabe"},
			},
		}
		result := orig.PackageMatches(other)
		if result {
			t.Errorf("expected <%t> got <%t>", false, result)
		}
	})

	t.Run("package added versions cause mismatch", func(t *testing.T) {
		other := api.BundlePackageSource{
			Registry:   "registry",
			Repository: "repository",
			Versions: []api.SourceVersion{
				{Name: "v1", Digest: "sha256:deadbeef"},
				{Name: "v2", Digest: "sha256:cafebabe"},
				{Name: "v3", Digest: "sha256:deadf00d"},
			},
		}
		result := orig.PackageMatches(other)
		if result {
			t.Errorf("expected <%t> got <%t>", false, result)
		}
	})

	t.Run("package removed versions cause mismatch", func(t *testing.T) {
		other := api.BundlePackageSource{
			Registry:   "registry",
			Repository: "repository",
			Versions: []api.SourceVersion{
				{Name: "v2", Digest: "sha256:cafebabe"},
			},
		}
		result := orig.PackageMatches(other)
		if result {
			t.Errorf("expected <%t> got <%t>", false, result)
		}
	})

	t.Run("package changed tags cause mismatch", func(t *testing.T) {
		other := api.BundlePackageSource{
			Registry:   "registry",
			Repository: "repository",
			Versions: []api.SourceVersion{
				{Name: "v1", Digest: "sha256:feedface"},
				{Name: "v2", Digest: "sha256:cafebabe"},
			},
		}
		result := orig.PackageMatches(other)
		if result {
			t.Errorf("expected <%t> got <%t>", false, result)
		}
	})
}

func TestSourceVersionKey(t *testing.T) {
	t.Parallel()

	s := api.SourceVersion{
		Name: "v1", Digest: "sha256:blah",
	}

	t.Run("smoke test", func(t *testing.T) {
		t.Parallel()

		if s.Key() != "v1 sha256:blah" {
			t.Errorf("smoke test")
		}
	})

	t.Run("includes the name", func(t *testing.T) {
		t.Parallel()

		got := s.Key()
		if !strings.Contains(got, "v1") {
			t.Errorf("expected key to contain the name, but it didn't")
		}
	})

	t.Run("includes the tag", func(t *testing.T) {
		t.Parallel()

		got := s.Key()
		if !strings.Contains(got, "sha256:blah") {
			t.Errorf("expected key to contain the tag, but it didn't")
		}
	})
}

func TestIsNewer(t *testing.T) {
	t.Parallel()

	givenBundle := func(name string) api.PackageBundle {
		return api.PackageBundle{ObjectMeta: metav1.ObjectMeta{Name: name}}
	}

	t.Run("less than", func(t *testing.T) {
		t.Parallel()

		current := givenBundle("v1-21-10002")
		candidate := givenBundle("v1-21-10003")
		if newer := current.LessThan(&candidate); !newer {
			t.Errorf("expected %v to be newer than %v", current.Name, candidate.Name)
		}
	})

	t.Run("greater than", func(t *testing.T) {
		t.Parallel()

		current := givenBundle("v1-21-10002")
		candidate := givenBundle("v1-21-10001")
		if newer := current.LessThan(&candidate); newer {
			t.Errorf("expected %v to not be newer than %v", current.Name, candidate.Name)
		}
	})

	t.Run("equal returns false", func(t *testing.T) {
		t.Parallel()

		current := givenBundle("v1-21-10002")
		candidate := givenBundle("v1-21-10002")
		if newer := current.LessThan(&candidate); newer {
			t.Errorf("expected %v to not be newer than %v", current.Name, candidate.Name)
		}
	})

	t.Run("newer kube major version", func(t *testing.T) {
		t.Parallel()

		current := givenBundle("v1-21-10002")
		candidate := givenBundle("v2-21-10002")
		if newer := current.LessThan(&candidate); !newer {
			t.Errorf("expected %v to be newer than %v", current.Name, candidate.Name)
		}
	})

	t.Run("newer kube minor version", func(t *testing.T) {
		t.Parallel()

		current := givenBundle("v1-21-10002")
		candidate := givenBundle("v1-22-10002")
		if newer := current.LessThan(&candidate); !newer {
			t.Errorf("expected %v to be newer than %v", current.Name, candidate.Name)
		}
	})
}

func TestGetPackageFromBundle(t *testing.T) {
	givenBundle := func(versions []api.SourceVersion) api.PackageBundle {
		return api.PackageBundle{
			Spec: api.PackageBundleSpec{
				Packages: []api.BundlePackage{
					{
						Name: "hello-eks-anywhere",
						Source: api.BundlePackageSource{
							Registry:   "public.ecr.aws/l0g8r8j6",
							Repository: "hello-eks-anywhere",
							Versions:   versions,
						},
					},
				},
			},
		}
	}

	t.Run("Get Package from bundle succeeds", func(t *testing.T) {

		bundle := givenBundle(
			[]api.SourceVersion{
				{
					Name:   "0.1.0",
					Digest: "sha256:eaa07ae1c06ffb563fe3c16cdb317f7ac31c8f829d5f1f32442f0e5ab982c3e7",
				},
			},
		)

		result, err := bundle.GetPackageFromBundle("hello-eks-anywhere")

		assert.Nil(t, err)
		assert.Equal(t, bundle.Spec.Packages[0].Name, result.Name)
	})

	t.Run("Get Package from bundle fails", func(t *testing.T) {

		bundle := givenBundle(
			[]api.SourceVersion{
				{
					Name:   "0.1.0",
					Digest: "sha256:eaa07ae1c06ffb563fe3c16cdb317f7ac31c8f829d5f1f32442f0e5ab982c3e7",
				},
			},
		)

		_, err := bundle.GetPackageFromBundle("harbor")

		assert.NotNil(t, err)
	})
}

func TestGetJsonSchemFromBundlePackage(t *testing.T) {
	givenBundle := func(versions []api.SourceVersion) api.PackageBundle {
		return api.PackageBundle{
			Spec: api.PackageBundleSpec{
				Packages: []api.BundlePackage{
					{
						Name: "hello-eks-anywhere",
						Source: api.BundlePackageSource{
							Versions: versions,
						},
					},
				},
			},
		}
	}

	t.Run("Get json schema from bundle succeeds", func(t *testing.T) {

		bundle := givenBundle(
			[]api.SourceVersion{
				{
					Schema: "H4sIAAAAAAAAA5VQvW7DIBDe/RQIdawh9ZgtqjplqZonuOCzTYIBHViRG+XdizGNImWoun7/d9eKMf6iW75lfIjRh62UAxrjajyHGux8GZBQeFBn6DGIhAoY4dtZuASh3CiDGnAEcQrO8tectiKPiQtZF6GjXrYEXZTNptnUb01JWM1RR4PZ+jSiCGafeXc8oYor5sl5pKgxJOaakIQFN5HCL+x1iDTf8YeEhGvb54SMt9jBZOJC+elotBKoSKQz5dOKog+KtI86HZ48h1zIqDSyzhErbxM8e26r9X7jfxbt8s/Zx/7Adn8teXc2grZILDf9tldlAYe21YsWzOfj4zowAatb9QNC+U5rEwIAAA==",
				},
			},
		)
		expected := "{\n  \"$id\": \"https://hello-eks-anywhere.packages.eks.amazonaws.com/schema.json\",\n  \"$schema\": \"https://json-schema.org/draft/2020-12/schema\",\n  \"title\": \"hello-eks-anywhere\",\n  \"type\": \"object\",\n  \"properties\": {\n    \"sourceRegistry\": {\n      \"type\": \"string\",\n      \"default\": \"public.ecr.aws/eks-anywhere\",\n      \"description\": \"Source registry for package.\"\n    },\n    \"title\": {\n      \"type\": \"string\",\n      \"default\": \"Amazon EKS Anywhere\",\n      \"description\": \"Container title.\"\n    }\n  },\n  \"additionalProperties\": false\n}\n"

		packageBundle := bundle.Spec.Packages[0]
		schema, err := packageBundle.GetJsonSchema()

		assert.Nil(t, err)
		assert.Equal(t, expected, string(schema))
	})

	t.Run("Get json schema from bundle fails when not compressed", func(t *testing.T) {
		bundle := givenBundle(
			[]api.SourceVersion{
				{
					Schema: "ewogICIkaWQiOiAiaHR0cHM6Ly9oZWxsby1la3MtYW55d2hlcmUucGFja2FnZXMuZWtzLmFtYXpvbmF3cy5jb20vc2NoZW1hLmpzb24iLAogICIkc2NoZW1hIjogImh0dHBzOi8vanNvbi1zY2hlbWEub3JnL2RyYWZ0LzIwMjAtMTIvc2NoZW1hIiwKICAidGl0bGUiOiAiaGVsbG8tZWtzLWFueXdoZXJlIiwKICAidHlwZSI6ICJvYmplY3QiLAogICJwcm9wZXJ0aWVzIjogewogICAgInNvdXJjZVJlZ2lzdHJ5IjogewogICAgICAidHlwZSI6ICJzdHJpbmciLAogICAgICAiZGVmYXVsdCI6ICJwdWJsaWMuZWNyLmF3cy9la3MtYW55d2hlcmUiLAogICAgICAiZGVzY3JpcHRpb24iOiAiU291cmNlIHJlZ2lzdHJ5IGZvciBwYWNrYWdlLiIKICAgIH0sCiAgICAidGl0bGUiOiB7CiAgICAgICJ0eXBlIjogInN0cmluZyIsCiAgICAgICJkZWZhdWx0IjogIkFtYXpvbiBFS1MgQW55d2hlcmUiLAogICAgICAiZGVzY3JpcHRpb24iOiAiQ29udGFpbmVyIHRpdGxlLiIKICAgIH0KICB9LAp9Cg==",
				},
			},
		)
		packageBundle := bundle.Spec.Packages[0]
		_, err := packageBundle.GetJsonSchema()

		assert.NotNil(t, err)
	})
}
