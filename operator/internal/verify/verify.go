// Package verify performs cosign signature verification of OCI module
// bundles. A ModuleSource may declare spec.verify (keyed or keyless); the
// operator then refuses to install a bundle that does not carry a matching
// signature, so a compromised registry can't serve a forged GameTemplate.
package verify

import (
	"context"
	"crypto"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	ggcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	ociremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	cosignsig "github.com/sigstore/cosign/v2/pkg/signature"
	"github.com/sigstore/sigstore/pkg/fulcioroots"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
)

// cosignPubKey is the Secret data key holding a keyed-verification public key.
const cosignPubKey = "cosign.pub"

// Verifier checks that the OCI artifact at ref@digest carries a valid cosign
// signature under a source's policy.
type Verifier interface {
	Verify(ctx context.Context, ref, digest string) error
}

// Nop accepts everything. Returned when a source declares no verify policy.
type Nop struct{}

// Verify always succeeds.
func (Nop) Verify(context.Context, string, string) error { return nil }

// Build constructs the Verifier for a source from spec.verify, returning Nop
// when verification is not configured. The public key and registry pull
// secret are resolved from namespace.
func Build(ctx context.Context, c client.Client, namespace string, src *kestrelv1alpha1.ModuleSource) (Verifier, error) {
	if src.Spec.Verify == nil {
		return Nop{}, nil
	}
	if src.Spec.OCI == nil {
		return nil, errors.New("spec.verify requires an oci source")
	}
	auth, err := authFor(ctx, c, namespace, src.Spec.OCI.PullSecretRef)
	if err != nil {
		return nil, err
	}
	insecure := src.Spec.OCI.Insecure
	v := src.Spec.Verify
	switch {
	case v.Key != nil:
		pub, err := readKey(ctx, c, namespace, v.Key.Name)
		if err != nil {
			return nil, err
		}
		return newKeyed(pub, auth, insecure)
	case v.Keyless != nil:
		return newKeyless(ctx, v.Keyless.Issuer, v.Keyless.Identity, auth, insecure)
	default:
		return nil, errors.New("spec.verify must set key or keyless")
	}
}

// cosignVerifier adapts the cosign library to the Verifier interface.
// mkOpts rebuilds CheckOpts per call so the request context flows into the
// registry client.
type cosignVerifier struct {
	mkOpts   func(ctx context.Context) *cosign.CheckOpts
	insecure bool
}

func (cv *cosignVerifier) Verify(ctx context.Context, ref, digest string) error {
	var opts []name.Option
	if cv.insecure {
		opts = append(opts, name.Insecure)
	}
	r, err := name.NewDigest(ref+"@"+digest, opts...)
	if err != nil {
		return fmt.Errorf("parse %s@%s: %w", ref, digest, err)
	}
	if _, _, err := cosign.VerifyImageSignatures(ctx, r, cv.mkOpts(ctx)); err != nil {
		return fmt.Errorf("cosign verify %s@%s: %w", ref, digest, err)
	}
	return nil
}

func baseCheckOpts(ctx context.Context, auth authn.Authenticator) *cosign.CheckOpts {
	return &cosign.CheckOpts{
		ClaimVerifier: cosign.SimpleClaimVerifier,
		RegistryClientOpts: []ociremote.Option{
			ociremote.WithRemoteOptions(
				ggcrremote.WithAuth(auth),
				ggcrremote.WithContext(ctx),
			),
		},
	}
}

func newKeyed(pubPEM []byte, auth authn.Authenticator, insecure bool) (Verifier, error) {
	ver, err := cosignsig.LoadPublicKeyRaw(pubPEM, crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("load cosign public key: %w", err)
	}
	mk := func(ctx context.Context) *cosign.CheckOpts {
		co := baseCheckOpts(ctx, auth)
		co.SigVerifier = ver
		// Keyed signatures need neither a transparency-log entry nor an SCT.
		co.IgnoreTlog = true
		co.IgnoreSCT = true
		co.Offline = true
		return co
	}
	return &cosignVerifier{mkOpts: mk, insecure: insecure}, nil
}

func newKeyless(ctx context.Context, issuer, identity string, auth authn.Authenticator, insecure bool) (Verifier, error) {
	roots, err := fulcioroots.Get()
	if err != nil {
		return nil, fmt.Errorf("load fulcio roots: %w", err)
	}
	intermediates, err := fulcioroots.GetIntermediates()
	if err != nil {
		return nil, fmt.Errorf("load fulcio intermediates: %w", err)
	}
	rekorPubs, err := cosign.GetRekorPubs(ctx)
	if err != nil {
		return nil, fmt.Errorf("load rekor public keys: %w", err)
	}
	ctPubs, err := cosign.GetCTLogPubs(ctx)
	if err != nil {
		return nil, fmt.Errorf("load ctlog public keys: %w", err)
	}
	mk := func(ctx context.Context) *cosign.CheckOpts {
		co := baseCheckOpts(ctx, auth)
		co.RootCerts = roots
		co.IntermediateCerts = intermediates
		co.RekorPubKeys = rekorPubs
		co.CTLogPubKeys = ctPubs
		co.Identities = []cosign.Identity{{Issuer: issuer, Subject: identity}}
		return co
	}
	return &cosignVerifier{mkOpts: mk, insecure: insecure}, nil
}

func readKey(ctx context.Context, c client.Client, namespace, name string) ([]byte, error) {
	var sec corev1.Secret
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &sec); err != nil {
		return nil, fmt.Errorf("get cosign key secret %s: %w", name, err)
	}
	pub, ok := sec.Data[cosignPubKey]
	if !ok || len(pub) == 0 {
		return nil, fmt.Errorf("secret %s has no %q data key", name, cosignPubKey)
	}
	return pub, nil
}

// authFor resolves a registry pull secret (kubernetes.io/dockerconfigjson)
// into a ggcr authenticator for fetching signatures, falling back to
// anonymous. A source's pull secret holds creds for its own registry, so the
// first auth entry is used without host matching.
func authFor(ctx context.Context, c client.Client, namespace string, ref *corev1.LocalObjectReference) (authn.Authenticator, error) {
	if ref == nil || ref.Name == "" {
		return authn.Anonymous, nil
	}
	var sec corev1.Secret
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: ref.Name}, &sec); err != nil {
		return nil, fmt.Errorf("get pull secret %s: %w", ref.Name, err)
	}
	cfg := sec.Data[corev1.DockerConfigJsonKey]
	if len(cfg) == 0 {
		return authn.Anonymous, nil
	}
	var dc struct {
		Auths map[string]struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Auth     string `json:"auth"`
		} `json:"auths"`
	}
	if err := json.Unmarshal(cfg, &dc); err != nil {
		return nil, fmt.Errorf("parse dockerconfigjson in %s: %w", ref.Name, err)
	}
	for _, e := range dc.Auths {
		switch {
		case e.Username != "" || e.Password != "":
			return authn.FromConfig(authn.AuthConfig{Username: e.Username, Password: e.Password}), nil
		case e.Auth != "":
			return authn.FromConfig(authn.AuthConfig{Auth: e.Auth}), nil
		}
	}
	return authn.Anonymous, nil
}
