package modsrc

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
	"github.com/kestrel-gg/kestrel/operator/internal/oci"
)

// Fetcher is one ModuleSource's view of its backing store. Index builds
// the catalog; Pull fetches one module bundle for installation.
//
// Index error contract: a non-nil error means the source as a whole is
// unreachable and no catalog should be published. Per-module failures
// are NOT errors — the failed module appears in the returned entries as
// a stub (Name + Reference, no versions) and a human-readable warning
// is appended, so one bad module never blanks the whole catalog.
type Fetcher interface {
	Index(ctx context.Context) (entries []kestrelv1alpha1.ModuleEntry, warnings []string, err error)
	Pull(ctx context.Context, name, version string) (*Bundle, error)
}

// Options carries operator-level configuration that individual fetchers
// need (set from CLI flags, not from the ModuleSource spec).
type Options struct {
	// LocalRoot is the base directory that local-type sources resolve
	// their relative paths under. Empty disables local sources.
	LocalRoot string
}

// ForSource builds the Fetcher for a ModuleSource. c and namespace are
// used to resolve credential Secrets living in the operator namespace.
func ForSource(ctx context.Context, c client.Client, namespace string, src *kestrelv1alpha1.ModuleSource, _ Options) (Fetcher, error) {
	creds, err := oci.CredentialFromSecret(ctx, c, namespace, src.Spec.PullSecretRef)
	if err != nil {
		return nil, fmt.Errorf("resolve credentials: %w", err)
	}
	names := make([]string, 0, len(src.Spec.Modules))
	for _, m := range src.Spec.Modules {
		names = append(names, m.Name)
	}
	return NewOCI(oci.New(creds, src.Spec.Insecure), src.Spec.URL, names), nil
}
