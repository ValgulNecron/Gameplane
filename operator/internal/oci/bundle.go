// Package oci wraps oras-go to pull Gameplane module bundles from any
// OCI-compliant registry. Bundles are single OCI artifacts whose layers
// carry the module metadata + GameTemplate manifest. This package is
// pure transport — bundle parsing and validation live in
// internal/modsrc, which consumes the raw layers returned by Pull.
package oci

// Media types for the Gameplane module artifact format. The full
// specification is in docs/module-authoring.md.
const (
	ArtifactType      = "application/vnd.gameplane.module.v1+json"
	MediaTypeConfig   = "application/vnd.gameplane.module.config.v1+json"
	MediaTypeMetadata = "application/vnd.gameplane.module.metadata.v1+yaml"
	MediaTypeTemplate = "application/vnd.gameplane.module.template.v1+yaml"
	MediaTypeReadme   = "application/vnd.gameplane.module.readme.v1+md"
	MediaTypeIconPNG  = "image/png"

	// AnnotationTitle is the OCI annotation that carries each layer's
	// filename inside the bundle (e.g. "module.yaml"). Pull keys the
	// returned layer map by this value; the canonical filenames are
	// modsrc.File* (the title annotation must match them exactly).
	AnnotationTitle = "org.opencontainers.image.title"

	// LayerNameMetadata, etc. duplicate the modsrc.File* canonical
	// filenames at the OCI layer-annotation level (this package cannot
	// import modsrc — modsrc wraps this client).
	LayerNameMetadata = "module.yaml"
	LayerNameTemplate = "template.yaml"
	LayerNameReadme   = "README.md"
	LayerNameIcon     = "icon.png"
)
