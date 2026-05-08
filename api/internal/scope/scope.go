// Package scope centralizes namespace selection and validation for
// handlers that touch the Kubernetes API.
//
// Trust model: the namespace a user is allowed to act in is derived
// from their role + any per-user scope (future work). For v1 we pin
// every action to `kestrel-games` unless an admin has explicitly
// opted into an extra namespace via the `KESTREL_EXTRA_NAMESPACES`
// env var. Anything else is rejected with 400.
//
// Never accept an un-validated namespace from a query string — that's
// CVE-bait. Everything goes through Resolve() or Allowed().
package scope

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/kestrel-gg/kestrel/api/internal/auth"
)

const DefaultNamespace = "kestrel-games"

// AllowedNamespaces returns the list of namespaces the API will act in.
// Computed once from env; small enough that a linear scan is fine.
var AllowedNamespaces = func() []string {
	extra := strings.Split(os.Getenv("KESTREL_EXTRA_NAMESPACES"), ",")
	out := []string{DefaultNamespace}
	for _, n := range extra {
		n = strings.TrimSpace(n)
		if n != "" && n != DefaultNamespace {
			out = append(out, n)
		}
	}
	return out
}()

// ErrForbiddenNamespace is returned when the requested namespace is
// not on the allow-list or the user's role doesn't permit it.
var ErrForbiddenNamespace = errors.New("namespace not permitted")

// Resolve returns the namespace to use for this request. If the
// request has no `namespace` query param, DefaultNamespace is used.
// Any provided value must be on AllowedNamespaces AND (for now) the
// caller must be at least operator — viewers are pinned to default.
func Resolve(req *http.Request) (string, error) {
	requested := strings.TrimSpace(req.URL.Query().Get("namespace"))
	if requested == "" {
		return DefaultNamespace, nil
	}
	if !contains(AllowedNamespaces, requested) {
		return "", ErrForbiddenNamespace
	}
	u := auth.UserFromContext(req.Context())
	// Viewers may only read the default namespace.
	if requested != DefaultNamespace && (u == nil || u.Role == "viewer") {
		return "", ErrForbiddenNamespace
	}
	return requested, nil
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
