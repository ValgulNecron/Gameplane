package handlers

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/ValgulNecron/gameplane/api/internal/auth"
	"github.com/ValgulNecron/gameplane/api/internal/kube"
	"github.com/ValgulNecron/gameplane/api/internal/scope"
)

const ownerOnlyTestOwnerID int64 = 7

// ownedServerObj returns a GameServer fixture owned by ownerOnlyTestOwnerID,
// plus any extra annotations.
func ownedServerObj(name string, extra map[string]string) *unstructured.Unstructured {
	obj := newServerObj("gameplane-games", name)
	ann := map[string]string{ownerIDAnnotation: strconv.FormatInt(ownerOnlyTestOwnerID, 10)}
	for k, v := range extra {
		ann[k] = v
	}
	obj.SetAnnotations(ann)
	return obj
}

// serverOwnerUser is the owner of ownedServerObj fixtures; it holds no
// namespace permissions.
func serverOwnerUser() *auth.User {
	return &auth.User{ID: ownerOnlyTestOwnerID, Username: "owner_carol", Role: "viewer"}
}

// namespaceAdminUser holds "*" only in the default namespace of the home cluster.
func namespaceAdminUser() *auth.User {
	return &auth.User{ID: 11, Username: "ns_admin", Role: "custom", Perms: map[string]map[string]map[string]struct{}{
		scope.DefaultCluster: {scope.DefaultNamespace: {"*": {}}},
	}}
}

// otherClusterAdminUser holds "*" only on a cluster other than the target.
func otherClusterAdminUser() *auth.User {
	return &auth.User{ID: 12, Username: "remote_admin", Role: "custom", Perms: map[string]map[string]map[string]struct{}{
		"other": {"*": {"*": {}}},
	}}
}

func serverAnnotations(t *testing.T, k *kube.Client, name string) map[string]string {
	t.Helper()
	obj, err := k.Dynamic.Resource(kube.GVRs["servers"]).
		Namespace("gameplane-games").Get(t.Context(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get %s: %v", name, err)
	}
	return obj.GetAnnotations()
}

func TestOwnership_TransferRequiresOwnerOrAdmin(t *testing.T) {
	store := newTestStore(t)
	target := seedUser(t, store, "dave", "viewer", "")
	k := fakeKubeClient(ownedServerObj("alpha", nil))
	reg := kube.NewRegistry(scope.DefaultCluster)
	reg.Set(scope.DefaultCluster, k)
	r := chi.NewRouter()
	MountOwnership(r, reg, store)
	body := map[string]any{"userId": target}

	for _, tc := range []struct {
		name string
		user *auth.User
		want int
	}{
		{"unauthenticated", nil, http.StatusUnauthorized},
		{"servers:write holder who is not the owner", testOperatorUser(), http.StatusForbidden},
		{"admin of another cluster only", otherClusterAdminUser(), http.StatusForbidden},
	} {
		t.Run(tc.name+" is refused", func(t *testing.T) {
			rr := doWithUser(t, r, "POST", "/servers/alpha:transfer", body, tc.user)
			if rr.Code != tc.want {
				t.Fatalf("got %d %s, want %d", rr.Code, rr.Body, tc.want)
			}
			if got := serverAnnotations(t, k, "alpha")[ownerIDAnnotation]; got != "7" {
				t.Fatalf("owner annotation = %q after a refused transfer, want 7", got)
			}
		})
	}

	t.Run("owner may transfer", func(t *testing.T) {
		rr := doWithUser(t, r, "POST", "/servers/alpha:transfer", body, serverOwnerUser())
		if rr.Code != http.StatusNoContent {
			t.Fatalf("got %d %s", rr.Code, rr.Body)
		}
		if got := serverAnnotations(t, k, "alpha")[ownerIDAnnotation]; got != strconv.FormatInt(target, 10) {
			t.Fatalf("owner annotation = %q, want %d", got, target)
		}
	})
	for _, tc := range []struct {
		name string
		user *auth.User
	}{
		{"cluster-wide admin", testAdminUser()},
		{"admin of the target namespace", namespaceAdminUser()},
	} {
		t.Run(tc.name+" may transfer a server it does not own", func(t *testing.T) {
			rr := doWithUser(t, r, "POST", "/servers/alpha:transfer", body, tc.user)
			if rr.Code != http.StatusNoContent {
				t.Fatalf("got %d %s", rr.Code, rr.Body)
			}
		})
	}
	t.Run("admin on a missing server gets 404", func(t *testing.T) {
		rr := doWithUser(t, r, "POST", "/servers/ghost:transfer", body, testAdminUser())
		if rr.Code != http.StatusNotFound {
			t.Fatalf("got %d %s", rr.Code, rr.Body)
		}
	})
}

func TestOwnership_SetCollaboratorsRequiresOwnerOrAdmin(t *testing.T) {
	store := newTestStore(t)
	erin := seedUser(t, store, "erin", "viewer", "")
	// testOperatorUser (ID 2) is already a collaborator on this server.
	k := fakeKubeClient(ownedServerObj("alpha", map[string]string{collaboratorsAnnotation: "2"}))
	reg := kube.NewRegistry(scope.DefaultCluster)
	reg.Set(scope.DefaultCluster, k)
	r := chi.NewRouter()
	MountOwnership(r, reg, store)
	body := map[string]any{"userIds": []int64{erin}}

	for _, tc := range []struct {
		name string
		user *auth.User
		want int
	}{
		{"unauthenticated", nil, http.StatusUnauthorized},
		{"collaborator holding servers:write", testOperatorUser(), http.StatusForbidden},
		{"admin of another cluster only", otherClusterAdminUser(), http.StatusForbidden},
	} {
		t.Run(tc.name+" is refused", func(t *testing.T) {
			rr := doWithUser(t, r, "PUT", "/servers/alpha:collaborators", body, tc.user)
			if rr.Code != tc.want {
				t.Fatalf("got %d %s, want %d", rr.Code, rr.Body, tc.want)
			}
			if got := serverAnnotations(t, k, "alpha")[collaboratorsAnnotation]; got != "2" {
				t.Fatalf("collaborators = %q after a refused edit, want 2", got)
			}
		})
	}

	for _, tc := range []struct {
		name string
		user *auth.User
	}{
		{"owner", serverOwnerUser()},
		{"cluster-wide admin", testAdminUser()},
		{"admin of the target namespace", namespaceAdminUser()},
	} {
		t.Run(tc.name+" may edit collaborators", func(t *testing.T) {
			rr := doWithUser(t, r, "PUT", "/servers/alpha:collaborators", body, tc.user)
			if rr.Code != http.StatusNoContent {
				t.Fatalf("got %d %s", rr.Code, rr.Body)
			}
			if got := serverAnnotations(t, k, "alpha")[collaboratorsAnnotation]; got != strconv.FormatInt(erin, 10) {
				t.Fatalf("collaborators = %q, want %d", got, erin)
			}
		})
	}
	t.Run("admin on a missing server gets 404", func(t *testing.T) {
		rr := doWithUser(t, r, "PUT", "/servers/ghost:collaborators", body, testAdminUser())
		if rr.Code != http.StatusNotFound {
			t.Fatalf("got %d %s", rr.Code, rr.Body)
		}
	})
}

func TestLifecycle_WipeDataRequiresOwnerOrAdmin(t *testing.T) {
	k := fakeKubeClient(ownedServerObj("alpha", nil), newServerObj("gameplane-games", "orphan"))
	r := mountLifecycleRouter(k)
	confirm := func(name string) map[string]any { return map[string]any{"confirm": name} }

	for _, tc := range []struct {
		name   string
		server string
		user   *auth.User
		want   int
	}{
		{"unauthenticated", "alpha", nil, http.StatusUnauthorized},
		{"servers:write holder who is not the owner", "alpha", testOperatorUser(), http.StatusForbidden},
		{"admin of another cluster only", "alpha", otherClusterAdminUser(), http.StatusForbidden},
		{"servers:write holder on a server with no owner", "orphan", testOperatorUser(), http.StatusForbidden},
		{"owner of another server on a server with no owner", "orphan", serverOwnerUser(), http.StatusForbidden},
	} {
		t.Run(tc.name+" is refused", func(t *testing.T) {
			rr := doWithUser(t, r, "POST", "/servers/"+tc.server+":wipe-data", confirm(tc.server), tc.user)
			if rr.Code != tc.want {
				t.Fatalf("got %d %s, want %d", rr.Code, rr.Body, tc.want)
			}
			if got := serverAnnotations(t, k, tc.server)[wipeRequestedAnnotation]; got != "" {
				t.Fatalf("wipe annotation set after a refused request: %q", got)
			}
		})
	}

	for _, tc := range []struct {
		name   string
		server string
		user   *auth.User
	}{
		{"owner", "alpha", serverOwnerUser()},
		{"cluster-wide admin", "alpha", testAdminUser()},
		{"admin of the target namespace", "alpha", namespaceAdminUser()},
		{"admin on a server with no owner", "orphan", testAdminUser()},
	} {
		t.Run(tc.name+" may wipe", func(t *testing.T) {
			rr := doWithUser(t, r, "POST", "/servers/"+tc.server+":wipe-data", confirm(tc.server), tc.user)
			if rr.Code != http.StatusAccepted {
				t.Fatalf("got %d %s", rr.Code, rr.Body)
			}
		})
	}
	t.Run("admin on a missing server gets 404", func(t *testing.T) {
		rr := doWithUser(t, r, "POST", "/servers/ghost:wipe-data", confirm("ghost"), testAdminUser())
		if rr.Code != http.StatusNotFound {
			t.Fatalf("got %d %s", rr.Code, rr.Body)
		}
	})
}
