package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/mod/semver"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

// Client is a thin wrapper around oras-go that knows how to enumerate
// tags and pull Gameplane module bundles.
type Client struct {
	credentials CredentialFunc
	insecure    bool
	httpClient  *http.Client
}

// New constructs a Client. credentials may be nil — anonymous pulls are
// then attempted. Pass insecure=true to use plain HTTP — intended for
// local kind/k3d registries that aren't behind TLS at all. Real
// registries with self-signed TLS should mount the CA bundle into the
// operator pod's trust store; this client deliberately does not skip
// TLS verification.
func New(credentials CredentialFunc, insecure bool) *Client {
	c := &Client{
		credentials: credentials,
		insecure:    insecure,
	}
	c.httpClient = &http.Client{Transport: retry.NewTransport(&http.Transport{})}
	return c
}

// repo builds an authenticated *remote.Repository pointing at ref. ref
// must include the registry hostname and repository path; tag/digest is
// optional (caller passes those to subsequent calls).
func (c *Client) repo(ref string) (*remote.Repository, error) {
	r, err := remote.NewRepository(ref)
	if err != nil {
		return nil, fmt.Errorf("parse reference %q: %w", ref, err)
	}
	r.PlainHTTP = c.insecure
	authClient := &auth.Client{
		Client: c.httpClient,
		Header: http.Header{"User-Agent": []string{"gameplane-operator/oci"}},
	}
	if c.credentials != nil {
		authClient.Credential = func(ctx context.Context, registry string) (auth.Credential, error) {
			return c.credentials(ctx, registry)
		}
	}
	r.Client = authClient
	return r, nil
}

// ListTags returns every tag under ref, semver-filtered and sorted in
// descending order. Non-semver tags (e.g. "latest", branch names) are
// dropped — modules are versioned, not pinned by mutable references.
func (c *Client) ListTags(ctx context.Context, ref string) ([]string, error) {
	r, err := c.repo(ref)
	if err != nil {
		return nil, err
	}
	var tags []string
	if err := r.Tags(ctx, "", func(page []string) error {
		tags = append(tags, page...)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("list tags %q: %w", ref, err)
	}
	versions := make([]string, 0, len(tags))
	for _, t := range tags {
		if semver.IsValid("v" + t) {
			versions = append(versions, t)
		}
	}
	sort.Slice(versions, func(i, j int) bool {
		return semver.Compare("v"+versions[i], "v"+versions[j]) > 0
	})
	return versions, nil
}

// Pull fetches the artifact at ref (which must include a tag or
// digest) and returns its manifest digest plus the layer blobs keyed
// by their title annotation. Untitled layers are skipped. Parsing the
// layers into a module bundle is the caller's job (modsrc.FromFiles).
func (c *Client) Pull(ctx context.Context, ref, reference string) (string, map[string][]byte, error) {
	r, err := c.repo(ref)
	if err != nil {
		return "", nil, err
	}
	manifestDesc, manifestRC, err := r.FetchReference(ctx, reference)
	if err != nil {
		return "", nil, fmt.Errorf("fetch %s@%s: %w", ref, reference, err)
	}
	defer manifestRC.Close()

	manifestBytes, err := io.ReadAll(manifestRC)
	if err != nil {
		return "", nil, fmt.Errorf("read manifest %s@%s: %w", ref, reference, err)
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return "", nil, fmt.Errorf("parse manifest %s@%s: %w", ref, reference, err)
	}
	if manifest.ArtifactType != "" && manifest.ArtifactType != ArtifactType {
		return "", nil, fmt.Errorf("unexpected artifactType %q at %s@%s; want %q",
			manifest.ArtifactType, ref, reference, ArtifactType)
	}

	files := map[string][]byte{}
	for _, layer := range manifest.Layers {
		title := layer.Annotations[AnnotationTitle]
		if title == "" {
			continue
		}
		blob, err := readBlob(ctx, r, layer)
		if err != nil {
			return "", nil, fmt.Errorf("read layer %s: %w", title, err)
		}
		files[title] = blob
	}
	return manifestDesc.Digest.String(), files, nil
}

func readBlob(ctx context.Context, r *remote.Repository, desc ocispec.Descriptor) ([]byte, error) {
	rc, err := r.Blobs().Fetch(ctx, desc)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
