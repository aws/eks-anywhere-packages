package v1alpha1_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	api "github.com/aws/modelrocket-add-ons/api/v1alpha1"
)

func TestPackageBundle_Find(t *testing.T) {
	var err error
	sut := api.PackageBundle{
		Spec: api.PackageBundleSpec{
			Packages: []api.BundlePackage{
				{
					Name: "eks-anywhere-test",
					Source: api.BundlePackageSource{
						Registry:   "public.ecr.aws/l0g8r8j6",
						Repository: "eks-anywhere-test",
						Versions: []api.SourceVersion{
							{
								Name: "v0.1.0",
								Tag:  "v0.1.0-c0c266629ccc70506ae818f282d12b01e287d2c6-helm",
							},
						},
					},
				},
			},
		},
	}

	expected := api.PackageOCISource{Registry: "public.ecr.aws/l0g8r8j6", Repository: "eks-anywhere-test", Tag: "v0.1.0-c0c266629ccc70506ae818f282d12b01e287d2c6-helm"}
	actual, err := sut.FindSource("eks-anywhere-test", "v0.1.0")
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)

	actual, err = sut.FindSource("eks-anywhere-test", "v0.1.0-c0c266629ccc70506ae818f282d12b01e287d2c6-helm")
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)

	expectedErr := "package not found: Bogus @ bar"
	_, err = sut.FindSource("Bogus", "bar")
	assert.EqualError(t, err, expectedErr)
}

func TestMatches(t *testing.T) {
	orig := api.BundlePackageSource{
		Registry:   "registry",
		Repository: "repository",
		Versions: []api.SourceVersion{
			{Name: "v1", Tag: "sha256:deadbeef"},
			{Name: "v2", Tag: "sha256:cafebabe"},
		},
	}

	t.Run("matches", func(t *testing.T) {
		other := api.BundlePackageSource{
			Registry:   "registry",
			Repository: "repository",
			Versions: []api.SourceVersion{
				{Name: "v1", Tag: "sha256:deadbeef"},
				{Name: "v2", Tag: "sha256:cafebabe"},
			},
		}
		result := orig.Matches(other)
		if !result {
			t.Errorf("expected <%t> got <%t>", true, result)
		}
	})

	t.Run("registries must match", func(t *testing.T) {
		other := api.BundlePackageSource{
			Registry:   "registry2",
			Repository: "repository",
			Versions: []api.SourceVersion{
				{Name: "v1", Tag: "sha256:deadbeef"},
				{Name: "v2", Tag: "sha256:cafebabe"},
			},
		}
		result := orig.Matches(other)
		if result {
			t.Errorf("expected <%t> got <%t>", false, result)
		}
	})

	t.Run("repositories must match", func(t *testing.T) {
		other := api.BundlePackageSource{
			Registry:   "registry",
			Repository: "repository2",
			Versions: []api.SourceVersion{
				{Name: "v1", Tag: "sha256:deadbeef"},
				{Name: "v2", Tag: "sha256:cafebabe"},
			},
		}
		result := orig.Matches(other)
		if result {
			t.Errorf("expected <%t> got <%t>", false, result)
		}
	})

	t.Run("added versions cause mismatch", func(t *testing.T) {
		other := api.BundlePackageSource{
			Registry:   "registry",
			Repository: "repository",
			Versions: []api.SourceVersion{
				{Name: "v1", Tag: "sha256:deadbeef"},
				{Name: "v2", Tag: "sha256:cafebabe"},
				{Name: "v3", Tag: "sha256:deadf00d"},
			},
		}
		result := orig.Matches(other)
		if result {
			t.Errorf("expected <%t> got <%t>", false, result)
		}
	})

	t.Run("removed versions cause mismatch", func(t *testing.T) {
		other := api.BundlePackageSource{
			Registry:   "registry",
			Repository: "repository",
			Versions: []api.SourceVersion{
				{Name: "v2", Tag: "sha256:cafebabe"},
			},
		}
		result := orig.Matches(other)
		if result {
			t.Errorf("expected <%t> got <%t>", false, result)
		}
	})

	t.Run("changed tags cause mismatch", func(t *testing.T) {
		other := api.BundlePackageSource{
			Registry:   "registry",
			Repository: "repository",
			Versions: []api.SourceVersion{
				{Name: "v1", Tag: "sha256:feedface"},
				{Name: "v2", Tag: "sha256:cafebabe"},
			},
		}
		result := orig.Matches(other)
		if result {
			t.Errorf("expected <%t> got <%t>", false, result)
		}
	})
}

func TestSourceVersionKey(t *testing.T) {
	t.Parallel()

	s := api.SourceVersion{
		Name: "v1", Tag: "sha256:blah",
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
