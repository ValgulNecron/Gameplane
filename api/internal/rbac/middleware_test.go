package rbac

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/ValgulNecron/gameplane/api/internal/auth"
)

func TestMiddleware_Unauthenticated(t *testing.T) {
	h := Middleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/servers", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d", rr.Code)
	}
}

// clusterWide builds a User holding perms as a cluster-wide ("*") binding.
func clusterWide(role string, perms ...string) *auth.User {
	set := map[string]struct{}{}
	for _, p := range perms {
		set[p] = struct{}{}
	}
	return &auth.User{ID: 1, Role: role, Perms: map[string]map[string]struct{}{"*": set}}
}

func TestMiddleware_AllowsViewerRead(t *testing.T) {
	called := false
	h := Middleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(204)
	}))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/servers", nil)
	req = req.WithContext(auth.WithUser(req.Context(), clusterWide(RoleViewer, "servers:read")))
	h.ServeHTTP(rr, req)
	if rr.Code != 204 || !called {
		t.Fatalf("code=%d called=%v", rr.Code, called)
	}
}

func TestMiddleware_DeniesViewerWrite(t *testing.T) {
	h := Middleware(nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("inner handler should not run")
	}))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/servers", nil)
	// Viewer holds reads but not servers:write.
	req = req.WithContext(auth.WithUser(req.Context(), clusterWide(RoleViewer, "servers:read")))
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("code=%d", rr.Code)
	}
}

func TestServerNameFromPath(t *testing.T) {
	cases := map[string]struct {
		path string
		name string
		ok   bool
	}{
		"simple":         {"/servers/alpha", "alpha", true},
		"with verb":      {"/servers/alpha:transfer", "alpha", true},
		"with subpath":   {"/servers/alpha/files", "alpha", true},
		"ws console":     {"/ws/servers/alpha/console", "alpha", true},
		"ws with verb":   {"/ws/servers/alpha:collaborators", "alpha", true},
		"list":           {"/servers", "", false},
		"empty name":     {"/servers/", "", false},
		"wrong segment":  {"/backups/alpha", "", false},
	}
	for label, tc := range cases {
		name, ok := serverNameFromPath(tc.path)
		if ok != tc.ok || name != tc.name {
			t.Errorf("%s: got (%q, %v) want (%q, %v)", label, name, ok, tc.name, tc.ok)
		}
	}
}

func TestFirstSegment(t *testing.T) {
	cases := map[string]string{
		"/":            "",
		"/servers":     "servers",
		"/servers/foo": "servers",
		"users":        "users",
	}
	for in, want := range cases {
		if got := firstSegment(in); got != want {
			t.Errorf("firstSegment(%q)=%q want %q", in, got, want)
		}
	}
}

// fakeServerFetcher mocks the ServerFetcher for tests.
type fakeServerFetcher map[string]*unstructured.Unstructured

func (f fakeServerFetcher) GetServer(ctx context.Context, ns, name string) (*unstructured.Unstructured, error) {
	if obj, ok := f[ns+"/"+name]; ok {
		return obj, nil
	}
	return nil, nil // not found (same as API not-found behavior)
}

func TestMiddleware_OwnerAllowedOnOwnedServer(t *testing.T) {
	// Owner should be allowed to read/write their own server even without namespace role.
	called := false
	h := Middleware(fakeServerFetcher{
		"gameplane-games/alpha": newOwnershipObj("alpha", 42, "alice", ""),
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(204)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/servers/alpha", nil)
	// User has no namespace permissions but is the owner.
	req = req.WithContext(auth.WithUser(req.Context(), &auth.User{ID: 42, Role: "viewer"}))
	h.ServeHTTP(rr, req)
	if rr.Code != 204 || !called {
		t.Fatalf("owner GET: code=%d called=%v", rr.Code, called)
	}
}

func TestMiddleware_CollaboratorAllowedOnServer(t *testing.T) {
	// Collaborator should be allowed on GET/POST/console but not on :transfer/:collaborators.
	called := false
	h := Middleware(fakeServerFetcher{
		"gameplane-games/alpha": newOwnershipObj("alpha", 42, "alice", "100,200"),
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(204)
	}))

	// User 100 is a collaborator.
	user := &auth.User{ID: 100, Role: "viewer"}

	// Test read allowed
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/servers/alpha", nil)
	req = req.WithContext(auth.WithUser(req.Context(), user))
	h.ServeHTTP(rr, req)
	if rr.Code != 204 || !called {
		t.Fatalf("collaborator GET: code=%d called=%v", rr.Code, called)
	}
}

func TestMiddleware_CollaboratorDeniedOnTransfer(t *testing.T) {
	// Collaborator should NOT be allowed on :transfer (owner-only).
	h := Middleware(fakeServerFetcher{
		"gameplane-games/alpha": newOwnershipObj("alpha", 42, "alice", "100,200"),
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler should not run")
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/servers/alpha:transfer", nil)
	req = req.WithContext(auth.WithUser(req.Context(), &auth.User{ID: 100}))
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("collaborator :transfer want 403 got %d", rr.Code)
	}
}

func TestMiddleware_OwnerAllowedOnTransfer(t *testing.T) {
	// Owner should be allowed on :transfer.
	called := false
	h := Middleware(fakeServerFetcher{
		"gameplane-games/alpha": newOwnershipObj("alpha", 42, "alice", "100"),
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(204)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/servers/alpha:transfer", nil)
	req = req.WithContext(auth.WithUser(req.Context(), &auth.User{ID: 42}))
	h.ServeHTTP(rr, req)
	if rr.Code != 204 || !called {
		t.Fatalf("owner :transfer: code=%d called=%v", rr.Code, called)
	}
}

func TestMiddleware_StrangerDenied(t *testing.T) {
	// A user who is neither owner nor collaborator should be denied.
	h := Middleware(fakeServerFetcher{
		"gameplane-games/alpha": newOwnershipObj("alpha", 42, "alice", "100"),
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler should not run")
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/servers/alpha", nil)
	req = req.WithContext(auth.WithUser(req.Context(), &auth.User{ID: 999}))
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("stranger want 403 got %d", rr.Code)
	}
}

func TestMiddleware_NilFetcher(t *testing.T) {
	// With nil fetcher, fallback is disabled; permission denial is final.
	h := Middleware(nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler should not run")
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/servers/alpha", nil)
	req = req.WithContext(auth.WithUser(req.Context(), &auth.User{ID: 42}))
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("nil fetcher want 403 got %d", rr.Code)
	}
}

func TestMiddleware_ListEndpointNoFallback(t *testing.T) {
	// List endpoints (no name) should not trigger the fallback.
	h := Middleware(fakeServerFetcher{
		"gameplane-games/alpha": newOwnershipObj("alpha", 42, "alice", ""),
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler should not run")
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/servers", nil)
	req = req.WithContext(auth.WithUser(req.Context(), &auth.User{ID: 42}))
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("list endpoint want 403 got %d", rr.Code)
	}
}

// newOwnershipObj creates a test GameServer object with owner and collaborators.
func newOwnershipObj(name string, ownerID int64, ownerName, collaborators string) *unstructured.Unstructured {
	ann := map[string]string{
		"gameplane.local/owner-id": strconv.FormatInt(ownerID, 10),
		"gameplane.local/owner":    ownerName,
	}
	if collaborators != "" {
		ann["gameplane.local/collaborators"] = collaborators
	}
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameServer",
		"metadata": map[string]any{
			"name":        name,
			"namespace":   "gameplane-games",
			"annotations": ann,
		},
	}}
	return obj
}
