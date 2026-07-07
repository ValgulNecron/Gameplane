package scope

import (
	"errors"
	"net/http"
	"strings"
)

// DefaultCluster is the ID of the always-present home cluster the API runs in.
// A single-cluster install only ever resolves to this, and requests that omit
// the `cluster` query param default to it.
const DefaultCluster = "local"

// ErrForbiddenCluster is returned when the requested cluster is not one of the
// currently-registered clusters.
var ErrForbiddenCluster = errors.New("cluster not permitted")

// ClusterLister is the subset of the kube.Registry API that ResolveCluster
// needs. It is declared here as an interface so this package does not import
// kube (which would risk an import cycle); *kube.Registry satisfies it.
type ClusterLister interface {
	IDs() []string
}

// ResolveCluster returns the target cluster ID for this request. If the
// request has no `cluster` query param, DefaultCluster is used. Any provided
// value must be one of the currently-registered clusters (reg.IDs()).
// This mirrors Resolve (namespace selection) exactly: which cluster a request
// targets, validated against the allow-list; per-caller authorization is a
// separate concern handled by the rbac middleware.
func ResolveCluster(req *http.Request, reg ClusterLister) (string, error) {
	requested := strings.TrimSpace(req.URL.Query().Get("cluster"))
	if requested == "" {
		return DefaultCluster, nil
	}
	for _, id := range reg.IDs() {
		if id == requested {
			return requested, nil
		}
	}
	return "", ErrForbiddenCluster
}
