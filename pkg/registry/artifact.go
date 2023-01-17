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

// ParseArtifactFromURI parses the URI into a new Artifact object.
func ParseArtifactFromURI(uri string) (*Artifact, error) {
	elements := strings.SplitN(uri, "/", 2)
	if len(elements) != 2 {
		return nil, fmt.Errorf("registry not found")
	}
	registry := elements[0]
	rol := elements[1]

	var tag string
	var digest string
	elements = strings.SplitN(rol, "@", 2)
	if len(elements) != 2 {
		elements = strings.SplitN(rol, ":", 2)
		if len(elements) != 2 {
			return nil, fmt.Errorf("tag or digest not found")
		}
		tag = elements[1]
	} else {
		digest = elements[1]
	}
	repository := elements[0]
	return &Artifact{
		Registry:   registry,
		Repository: repository,
		Tag:        tag,
		Digest:     digest,
	}, nil
}

// Version returns tag or digest.
func (art Artifact) Version() string {
	if art.Digest != "" {
		return "@" + art.Digest
	}
	return ":" + art.Tag
}

// VersionedImage returns full URI for image.
func (art Artifact) VersionedImage() string {
	version := art.Version()
	return art.Registry + "/" + art.Repository + version
}
