package kube

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Registry holds a pool of Kubernetes clients keyed by cluster ID, so the API
// can dispatch each request to the right target cluster. A single-cluster
// install has exactly one entry: the default "local" cluster. The zero value
// is not usable; construct with NewRegistry. All methods are safe for
// concurrent use.
type Registry struct {
	mu        sync.RWMutex
	clients   map[string]*Client
	defaultID string
}

// NewRegistry returns an empty Registry whose default cluster is defaultID.
func NewRegistry(defaultID string) *Registry {
	return &Registry{clients: map[string]*Client{}, defaultID: defaultID}
}

// Get returns the client for cluster id and true, or (nil, false) if the
// cluster is not registered.
func (r *Registry) Get(id string) (*Client, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.clients[id]
	return c, ok
}

// Default returns the client for the registry's default cluster, or nil if it
// has not been registered.
func (r *Registry) Default() *Client {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.clients[r.defaultID]
}

// DefaultID returns the ID of the default cluster.
func (r *Registry) DefaultID() string {
	return r.defaultID
}

// Set registers (or replaces) the client for cluster id.
func (r *Registry) Set(id string, c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[id] = c
}

// Remove deletes the client for cluster id, if present.
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, id)
}

// IDs returns the registered cluster IDs in sorted order.
func (r *Registry) IDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.clients))
	for id := range r.clients {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// GetServer fetches a GameServer from the named cluster, so callers like the
// rbac middleware's owner/collaborator fallback can target the right cluster.
// Returns an error if the cluster is not registered.
func (r *Registry) GetServer(ctx context.Context, cluster, ns, name string) (*unstructured.Unstructured, error) {
	c, ok := r.Get(cluster)
	if !ok {
		return nil, fmt.Errorf("cluster %q not registered", cluster)
	}
	return c.GetServer(ctx, ns, name)
}
