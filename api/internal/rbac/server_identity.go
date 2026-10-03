package rbac

import (
	"context"
	"errors"
)

// ServerIdentity is the exact resource that granted owner/collaborator access.
type ServerIdentity struct {
	Cluster   string
	Namespace string
	Name      string
	UID       string
}

type serverIdentityKey struct{}

// ErrServerIdentityChanged rejects a resource different from the one authorized.
var ErrServerIdentityChanged = errors.New("authorized server identity changed")

// BoundServerIdentity returns the immutable identity used by ownership fallback.
// Namespace-wide permission grants are not restricted to a single resource UID.
func BoundServerIdentity(ctx context.Context) (ServerIdentity, bool) {
	identity, ok := ctx.Value(serverIdentityKey{}).(ServerIdentity)
	return identity, ok
}

// ValidateServerIdentity prevents a later lookup from silently rebinding an
// ownership grant to a replacement server. Unbound namespace permissions retain
// their existing behavior because they authorize every server in that scope.
func ValidateServerIdentity(ctx context.Context, cluster, namespace, name, uid string) error {
	bound, ok := BoundServerIdentity(ctx)
	if !ok {
		return nil
	}
	if bound.UID == "" || bound != (ServerIdentity{Cluster: cluster, Namespace: namespace, Name: name, UID: uid}) {
		return ErrServerIdentityChanged
	}
	return nil
}
