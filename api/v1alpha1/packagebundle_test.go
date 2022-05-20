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

		result := bundle.KubeVersionMatches(targetVersion)

		if !result {
			t.Errorf("expected <%t> got <%t>", true, result)
		}
	})

	t.Run("Kubernetes major version doesn't match", func(t *testing.T) {

		targetVersion := "v2-21-1"

		result := bundle.KubeVersionMatches(targetVersion)

		if result {
			t.Errorf("expected <%t> got <%t>", false, result)
		}
	})

	t.Run("Kubernetes minor version doesn't match", func(t *testing.T) {

		targetVersion := "v1-22-1"

		result := bundle.KubeVersionMatches(targetVersion)

		if result {
			t.Errorf("expected <%t> got <%t>", false, result)
		}
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
