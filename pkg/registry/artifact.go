package registry

import (
	"fmt"
	"strings"
)

// Artifact to head release dependency.
type Artifact struct {
	Registry   string
	Repository string
	Tag        string
	Digest     string
}

// NewArtifact creates a new artifact object.
func NewArtifact(registry, repository, tag, digest string) Artifact {
	return Artifact{
		Registry:   registry,
		Repository: repository,
		Tag:        tag,
		Digest:     digest,
	}
}

// SetURI value for artifact.
func (art *Artifact) SetURI(uri string) error {
	elements := strings.SplitN(uri, "/", 2)
	if len(elements) != 2 {
		return fmt.Errorf("registry not found")
	}
	registry := elements[0]
	rol := elements[1]

	var tag string
	var digest string
	elements = strings.SplitN(rol, "@", 2)
	if len(elements) != 2 {
		elements = strings.SplitN(rol, ":", 2)
		if len(elements) != 2 {
			return fmt.Errorf("tag or digest not found")
		}
		tag = elements[1]
	} else {
		digest = elements[1]
	}
	repository := elements[0]
	art.Registry = registry
	art.Repository = repository
	art.Tag = tag
	art.Digest = digest
	return nil
}

// Version returns tag or digest.
func (art *Artifact) Version() string {
	if art.Digest != "" {
		return "@" + art.Digest
	}
	return ":" + art.Tag
}

// VersionedImage returns full URI for image.
func (art *Artifact) VersionedImage() string {
	version := art.Version()
	return art.Registry + "/" + art.Repository + version
}
