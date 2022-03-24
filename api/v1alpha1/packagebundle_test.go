package v1alpha1_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

func TestPackageBundle_Find(t *testing.T) {
	var err error
	givenBundle := func(versions []api.SourceVersion) api.PackageBundle {
		return api.PackageBundle{
			Spec: api.PackageBundleSpec{
				Packages: []api.BundlePackage{
					{
						Name: "eks-anywhere-test",
						Source: api.BundlePackageSource{
							Registry:   "public.ecr.aws/l0g8r8j6",
							Repository: "eks-anywhere-test",
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
		Repository: "eks-anywhere-test",
		Digest:     "sha256:eaa07ae1c06ffb563fe3c16cdb317f7ac31c8f829d5f1f32442f0e5ab982c3e7",
		Version:    "0.1.0",
	}

	actual, err := sut.FindSource("eks-anywhere-test", "0.1.0")
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)

	actual, err = sut.FindSource("eks-anywhere-test", "sha256:eaa07ae1c06ffb563fe3c16cdb317f7ac31c8f829d5f1f32442f0e5ab982c3e7")
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
			Repository: "eks-anywhere-test",
			Digest:     "sha256:deadbeef",
			Version:    "0.1.1",
		}
		actual, err = latest.FindSource("eks-anywhere-test", api.Latest)
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
			Repository: "eks-anywhere-test",
			Digest:     "sha256:eaa07ae1c06ffb563fe3c16cdb317f7ac31c8f829d5f1f32442f0e5ab982c3e7",
			Version:    "0.1.0",
		}
		actual, err = latest.FindSource("eks-anywhere-test", api.Latest)
		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	})
}

func TestMatches(t *testing.T) {
	orig := api.BundlePackageSource{
		Registry:   "registry",
		Repository: "repository",
		Versions: []api.SourceVersion{
			{Name: "v1", Digest: "sha256:deadbeef"},
			{Name: "v2", Digest: "sha256:cafebabe"},
		},
	}

	t.Run("matches", func(t *testing.T) {
		other := api.BundlePackageSource{
			Registry:   "registry",
			Repository: "repository",
			Versions: []api.SourceVersion{
				{Name: "v1", Digest: "sha256:deadbeef"},
				{Name: "v2", Digest: "sha256:cafebabe"},
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
				{Name: "v1", Digest: "sha256:deadbeef"},
				{Name: "v2", Digest: "sha256:cafebabe"},
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
				{Name: "v1", Digest: "sha256:deadbeef"},
				{Name: "v2", Digest: "sha256:cafebabe"},
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
				{Name: "v1", Digest: "sha256:deadbeef"},
				{Name: "v2", Digest: "sha256:cafebabe"},
				{Name: "v3", Digest: "sha256:deadf00d"},
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
				{Name: "v2", Digest: "sha256:cafebabe"},
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
				{Name: "v1", Digest: "sha256:feedface"},
				{Name: "v2", Digest: "sha256:cafebabe"},
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
