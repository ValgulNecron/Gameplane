// Package oci wraps oras-go to pull Kestrel module bundles from any
// OCI-compliant registry. Bundles are single OCI artifacts whose layers
// carry the module metadata + GameTemplate manifest.
package oci

// Media types for the Kestrel module artifact format. The full
// specification is in docs/module-authoring.md.
const (
	ArtifactType      = "application/vnd.kestrel.module.v1+json"
	MediaTypeConfig   = "application/vnd.kestrel.module.config.v1+json"
	MediaTypeMetadata = "application/vnd.kestrel.module.metadata.v1+yaml"
	MediaTypeTemplate = "application/vnd.kestrel.module.template.v1+yaml"
	MediaTypeReadme   = "application/vnd.kestrel.module.readme.v1+md"
	MediaTypeIconPNG  = "image/png"

	// AnnotationTitle is the OCI annotation that carries each layer's
	// filename inside the bundle (e.g. "module.yaml"). We use it as the
	// primary disambiguator since artifactType is the same for every
	// layer of the same kind.
	AnnotationTitle = "org.opencontainers.image.title"

	// LayerNameMetadata, etc. are the canonical filenames each layer
	// must carry in its title annotation. Pullers use these to locate
	// layers by purpose.
	LayerNameMetadata = "module.yaml"
	LayerNameTemplate = "template.yaml"
	LayerNameReadme   = "README.md"
	LayerNameIcon     = "icon.png"
)

// Metadata mirrors the on-disk module.yaml schema documented in
// docs/module-authoring.md. Fields not listed here are ignored.
type Metadata struct {
	APIVersion        string `yaml:"apiVersion" json:"apiVersion"`
	Name              string `yaml:"name" json:"name"`
	DisplayName       string `yaml:"displayName" json:"displayName"`
	Version           string `yaml:"version" json:"version"`
	Game              string `yaml:"game" json:"game"`
	Summary           string `yaml:"summary,omitempty" json:"summary,omitempty"`
	Homepage          string `yaml:"homepage,omitempty" json:"homepage,omitempty"`
	License           string `yaml:"license,omitempty" json:"license,omitempty"`
	KestrelMinVersion string `yaml:"kestrelMinVersion,omitempty" json:"kestrelMinVersion,omitempty"`
	Icon              string `yaml:"icon,omitempty" json:"icon,omitempty"`
}

// Bundle is the parsed contents of one module artifact, addressed by the
// digest of its manifest.
type Bundle struct {
	// Digest is the manifest digest of the bundle (e.g.
	// "sha256:…"). Stamped onto the resulting GameTemplate so a Module
	// reconcile can detect drift even if the tag is reused.
	Digest string

	// Metadata is the parsed module.yaml.
	Metadata Metadata

	// TemplateYAML is the raw GameTemplate manifest YAML (without
	// metadata.name set). The reconciler decodes this into a v1alpha1
	// GameTemplate when materializing the install.
	TemplateYAML []byte

	// Readme is the raw README.md bytes, or nil when the bundle did not
	// ship one.
	Readme []byte

	// Icon is the raw icon bytes when the bundle ships an icon layer.
	// Currently surfaced via the API by base64-encoding into a data
	// URI, sized down by the caller.
	Icon []byte
}
