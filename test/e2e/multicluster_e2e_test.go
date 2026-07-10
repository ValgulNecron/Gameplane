//go:build e2e

// Dual-cluster coverage for the multi-cluster `?cluster=` request dispatch
// (api/internal/scope.ResolveCluster, api/internal/kube.Registry) and the
// RBAC cluster dimension (api/internal/rbac, migration
// 004_cluster_rbac.sql). Every other test in this package runs against the
// single kind cluster the harness in env.go/e2e_suite_test.go stands up
// ("cluster A" / envInstance, registered under the API's fixed id "local").
// This file additionally requires a SECOND, independent kind cluster
// ("cluster B") to prove a GameServer created via `?cluster=<B>` actually
// lands on the other cluster's Kubernetes API — not just behind a query
// param that the handler ignores — and that a viewer bound only to "local"
// cannot see it.
//
// # Infra this test assumes
//
//   - A second kind cluster is already up, with the same Gameplane chart
//     installed (CRDs + operator: cluster B needs its own operator to
//     reconcile the GameServer this test creates there, per "the operator
//     is authoritative" — see CLAUDE.md rule 10 and
//     docs/install.md#registering-an-additional-cluster). It must be
//     reachable at kubeconfig context "kind-<name>", where <name> comes
//     from GAMEPLANE_E2E_CLUSTER_B (default "gameplane-e2e-b").
//   - The dedicated CI job "e2e-multicluster" in .github/workflows/ci.yaml
//     brings this up by calling `deploy/kind/e2e.sh up gameplane-e2e-b e2e`
//     — the exact same script and image tag as the primary cluster — right
//     after standing up cluster A the normal way.
//   - `kind` and `docker` CLIs on PATH (already required transitively by
//     deploy/kind/e2e.sh).
//
// # Why a raw docker-network IP, not the kubeconfig `kind` hands out
//
// Registering cluster B (POST /clusters) needs a kubeconfig that is
// reachable from INSIDE cluster A's gameplane-api (and, for the health
// reconciler, gameplane-operator) pods — not from the CI runner's own
// network namespace. `kind get kubeconfig` (no flags) points at
// 127.0.0.1:<host-mapped-port>, which is only valid from the runner itself.
// `kind get kubeconfig --internal` instead names the server by the node
// container's hostname (e.g. https://gameplane-e2e-b-control-plane:6443),
// but that hostname is only resolvable via Docker's embedded DNS — kind
// deliberately strips loopback-pointing nameservers from each node's
// resolv.conf, so cluster A's in-pod CoreDNS forwards upstream and never
// reaches it either.
//
// What DOES work: every kind cluster's nodes attach to the same docker
// bridge network ("kind" by default), so cluster B's control-plane
// container has a real, directly-routable IP address on that bridge that
// cluster A's nodes (and therefore its pods, via the node's own egress
// path) can reach at the network layer. podReachableKubeconfig below takes
// the `--internal` kubeconfig (which already carries the right client
// certificate for cluster-admin auth) and swaps the server host for that
// docker-network IP, skipping server-certificate verification since the
// IP is not guaranteed to be one of the API server certificate's SANs —
// client authentication (the part that actually gates access) is
// unaffected by that flag.
//
// If the second cluster/context isn't present — e.g. iterating locally
// against just the single-cluster harness — the test SKIPS rather than
// faking coverage; see ensureClusterB.
package e2e

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

// clusterBKindName is the kind cluster name the CI job "e2e-multicluster"
// stands up as the second, remote-registered cluster. Overridable via
// GAMEPLANE_E2E_CLUSTER_B for local iteration against a differently-named
// second cluster.
func clusterBKindName() string {
	return getenvOr("GAMEPLANE_E2E_CLUSTER_B", "gameplane-e2e-b")
}

// ensureClusterB connects directly to the second kind cluster's Kubernetes
// API (bypassing the Gameplane API entirely), so the test can assert
// ground truth on each cluster independently in addition to dispatching
// through the control-plane API. It skips the test — not a failure — when
// the second cluster isn't configured or reachable: this dual-cluster
// setup is opt-in infra layered on top of the single-cluster harness, not
// part of the default suite every bucket gets for free.
func ensureClusterB(t *testing.T) (envB *Env, name string) {
	t.Helper()
	name = clusterBKindName()
	envB, err := newEnvForContext("kind-" + name)
	if err != nil {
		t.Skipf("second kind cluster %q not configured (kubeconfig context kind-%s): %v — "+
			"this test needs the e2e-multicluster CI job's dual-cluster setup "+
			"(see the package doc in multicluster_e2e_test.go)", name, name, err)
	}
	if err := envB.ensureCluster(); err != nil {
		t.Skipf("second kind cluster %q not reachable: %v — bring it up with "+
			"`deploy/kind/e2e.sh up %s e2e` before running this test", name, err, name)
	}
	return envB, name
}

// podReachableKubeconfig returns a kubeconfig for the kind cluster named
// clusterName that is dialable from a POD running in a DIFFERENT kind
// cluster — see the package doc above for why neither of `kind`'s own two
// kubeconfig flavors (default, --internal) works as-is for that. It shells
// out to `kind` and `docker`, both already required by deploy/kind/e2e.sh.
func podReachableKubeconfig(t *testing.T, clusterName string) []byte {
	t.Helper()

	raw, err := exec.Command("kind", "get", "kubeconfig", "--internal", "--name", clusterName).Output()
	if err != nil {
		t.Fatalf("kind get kubeconfig --internal --name %s: %v", clusterName, exitErr(err))
	}

	containerName := clusterName + "-control-plane"
	// Target the "kind" network by name rather than taking the first network
	// in map iteration order: `docker inspect` with a bare {{range .Networks}}
	// returns whichever network Go's (unordered) map iteration visits first,
	// which silently picks the wrong IP on a multi-homed container (e.g. one
	// also attached to the local OCI registry's network from `make dev-up`,
	// or another docker-compose network on a shared runner). Every kind node
	// is guaranteed to be on "kind" (see the package doc above), so name it
	// explicitly.
	ipOut, err := exec.Command("docker", "inspect", "-f",
		`{{(index .NetworkSettings.Networks "kind").IPAddress}}`, containerName).Output()
	if err != nil {
		t.Fatalf("docker inspect %s: %v", containerName, exitErr(err))
	}
	ip := strings.TrimSpace(string(ipOut))
	if ip == "" {
		t.Fatalf("docker inspect %s: empty IP address on the \"kind\" network", containerName)
	}

	cfg, err := clientcmd.Load(raw)
	if err != nil {
		t.Fatalf("parse internal kubeconfig for %s: %v", clusterName, err)
	}
	for _, c := range cfg.Clusters {
		c.Server = fmt.Sprintf("https://%s:6443", ip)
		// The docker-network IP isn't guaranteed to be a SAN on the API
		// server certificate; skip server-cert verification rather than
		// depend on it. This only weakens verification of the SERVER's
		// identity over a docker-bridge-local hop inside a throwaway CI
		// cluster — the client certificate below (real mTLS auth issued by
		// cluster B's own CA) is what actually gates access, and that is
		// unaffected.
		c.InsecureSkipTLSVerify = true
		c.CertificateAuthority = ""
		c.CertificateAuthorityData = nil
	}
	out, err := clientcmd.Write(*cfg)
	if err != nil {
		t.Fatalf("re-marshal kubeconfig for %s: %v", clusterName, err)
	}
	return out
}

// exitErr enriches an *exec.ExitError with its captured stderr, which
// exec.Command's default Output() error otherwise discards.
func exitErr(err error) error {
	if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
		return fmt.Errorf("%w: %s", err, string(ee.Stderr))
	}
	return err
}

// TestMultiCluster_ClusterDispatchAndScopedRBAC registers a second, real
// kind cluster as a remote target (POST /clusters), creates a GameServer on
// it through cluster A's API via `?cluster=<B>`, and proves three things
// that a single-cluster suite can never actually exercise:
//
//  1. The GameServer lands on cluster B's own Kubernetes API — verified by
//     reading it directly from cluster B, bypassing the Gameplane API — and
//     is genuinely absent from cluster A, not merely filtered from a shared
//     list by the query param.
//  2. A viewer whose only role binding is on cluster "local" (the default
//     CreateUser binding — see api/internal/handlers/users.go's
//     SetClusterRoleBinding(..., scope.DefaultCluster, ...)) is forbidden
//     from reading cluster B's servers: the RBAC cluster dimension actually
//     gates namespaced permissions per target cluster, not just per
//     namespace.
//  3. `?cluster=<unknown>` is rejected as a bad request (400) for any
//     caller, before RBAC or the handler ever sees a namespace/name.
func TestMultiCluster_ClusterDispatchAndScopedRBAC(t *testing.T) {
	t.Parallel()

	envB, clusterBName := ensureClusterB(t)

	clusterID := fmt.Sprintf("e2e-mc-b-%d", time.Now().UnixNano())

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	admin := envInstance.APIClient(t, adminUsername, adminPassword)
	defer admin.Close()

	// --- Register cluster B as a remote target ------------------------------
	kubeconfig := podReachableKubeconfig(t, clusterBName)
	resp, body, err := admin.Post("/clusters", map[string]string{
		"name":        clusterID,
		"displayName": "E2E cluster B",
		"kubeconfig":  string(kubeconfig),
	})
	if err != nil {
		t.Fatalf("POST /clusters: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /clusters: status=%d body=%s", resp.StatusCode, string(body))
	}
	// Registered first (and so, by t.Cleanup's LIFO order, torn down LAST) —
	// the GameServer/GameTemplate cleanups below dispatch through this
	// cluster registration and must run while it still resolves.
	t.Cleanup(func() {
		_, _, _ = admin.Delete("/clusters/" + clusterID)
	})

	// The API's cluster watch (kube.WatchClusters) loads a client for the new
	// Cluster CR asynchronously off an informer Add event; wait for it to
	// show up in the registry (surfaced via GET /clusters) before dispatching
	// anything to it, or the create below could race a registry that hasn't
	// caught up yet.
	envInstance.Eventually(t, 60*time.Second, func() (bool, string) {
		resp, body, err := admin.Get("/clusters")
		if err != nil {
			return false, err.Error()
		}
		if resp.StatusCode != http.StatusOK {
			return false, fmt.Sprintf("GET /clusters: status=%d body=%s", resp.StatusCode, string(body))
		}
		if strings.Contains(string(body), clusterID) {
			return true, ""
		}
		return false, "cluster not yet listed: " + string(body)
	})

	// --- Create a GameTemplate + GameServer directly on cluster B -----------
	tmplName := fmt.Sprintf("e2e-mc-tmpl-%d", time.Now().UnixNano())
	resp, body, err = admin.Post("/templates?cluster="+clusterID, map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameTemplate",
		"metadata":   map[string]any{"name": tmplName},
		"spec": map[string]any{
			"displayName": "E2E busybox (cluster B)",
			"game":        "busybox",
			"version":     "1",
			"image":       "busybox:1.36",
			"command":     []any{"sh", "-c", "sleep 100000"},
			"ports": []any{
				map[string]any{"name": "noop", "containerPort": int64(12345), "advertise": true, "protocol": "TCP"},
			},
		},
	})
	if err != nil {
		t.Fatalf("POST /templates?cluster=%s: %v", clusterID, err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /templates?cluster=%s: status=%d body=%s", clusterID, resp.StatusCode, string(body))
	}
	t.Cleanup(func() {
		_, _, _ = admin.Delete("/templates/" + tmplName + "?cluster=" + clusterID)
	})

	const ns = "gameplane-games"

	// --- A dedicated operator user for cluster-B's namespaced writes --------
	// servers:read/servers:write are NAMESPACED perms: auth.User.Can gates
	// them on the request's target cluster (or an explicit cluster="*"
	// binding) — a binding on some OTHER cluster never confers them, by
	// design (see the cluster-gating comment on Can). The bootstrap admin's
	// only binding is (cluster=local, namespace=*, role=admin), so
	// admin.Post("/servers?cluster="+clusterID, ...) would 403, not 201.
	// (POST /templates above is unaffected: templates:* is cluster-scoped,
	// not namespaced, so it's granted by admin's cluster-wide binding on ANY
	// cluster.)
	//
	// Grant a dedicated user an explicit (cluster=B, namespace=gameplane-
	// games) binding instead of widening the admin's own grants, so this
	// test doesn't depend on (or risk masking a regression in) cross-cluster
	// admin scoping. It's an "operator" primary role, not another admin, to
	// keep the elevated grant scoped to exactly the namespace/cluster this
	// test needs.
	opUsername, opPassword, opID := envInstance.CreateUser(t, admin, "operator", "e2e-mc-operator")
	t.Cleanup(func() {
		_, _, _ = admin.Delete("/users/" + opID)
	})
	resp, body, err = admin.Post("/users/"+opID+"/bindings", map[string]any{
		"roleName":  "admin",
		"cluster":   clusterID,
		"namespace": ns,
	})
	if err != nil {
		t.Fatalf("POST /users/%s/bindings: %v", opID, err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /users/%s/bindings: status=%d body=%s", opID, resp.StatusCode, string(body))
	}
	// addBinding invalidates the target user's existing sessions (see
	// userHandler.invalidateSessions) so a bound-in-flight session wouldn't
	// see the new grant anyway — log in AFTER granting the binding, not
	// before, so the resulting session is minted post-binding instead of
	// being torn down by it.
	operatorClient := envInstance.APIClient(t, opUsername, opPassword)
	defer operatorClient.Close()

	gsName := fmt.Sprintf("e2e-mc-gs-%d", time.Now().UnixNano())
	resp, body, err = operatorClient.Post("/servers?cluster="+clusterID, map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameServer",
		"metadata":   map[string]any{"name": gsName, "namespace": ns},
		"spec":       map[string]any{"templateRef": map[string]any{"name": tmplName}},
	})
	if err != nil {
		t.Fatalf("POST /servers?cluster=%s: %v", clusterID, err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /servers?cluster=%s: status=%d body=%s", clusterID, resp.StatusCode, string(body))
	}
	t.Cleanup(func() {
		_, _, _ = operatorClient.Delete("/servers/" + gsName + "?cluster=" + clusterID)
	})

	// --- Ground truth: read each cluster's own Kubernetes API directly,  ----
	// --- entirely bypassing the Gameplane API.                           ----
	if _, err := envB.Dyn.Resource(gameServerGVR).Namespace(ns).
		Get(context.Background(), gsName, metav1.GetOptions{}); err != nil {
		t.Fatalf("gameserver %s/%s not found directly on cluster B: %v", ns, gsName, err)
	}
	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Get(context.Background(), gsName, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("gameserver %s/%s unexpectedly reachable directly on cluster A (err=%v)", ns, gsName, err)
	}

	// --- Through the API: cluster=B lists it, the default cluster doesn't --
	// Uses operatorClient, not admin: this is the same namespaced
	// servers:read check as the POST above, gated the same way.
	resp, body, err = operatorClient.Get("/servers?cluster=" + clusterID)
	if err != nil {
		t.Fatalf("GET /servers?cluster=%s: %v", clusterID, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /servers?cluster=%s: status=%d body=%s", clusterID, resp.StatusCode, string(body))
	}
	if !strings.Contains(string(body), gsName) {
		t.Errorf("GET /servers?cluster=%s: expected %s in listing, got %s", clusterID, gsName, string(body))
	}

	resp, body, err = admin.Get("/servers")
	if err != nil {
		t.Fatalf("GET /servers (default cluster): %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /servers (default cluster): status=%d body=%s", resp.StatusCode, string(body))
	}
	if strings.Contains(string(body), gsName) {
		t.Errorf("GET /servers (default cluster): cluster-B server %s leaked into the local listing: %s",
			gsName, string(body))
	}

	// --- RBAC: a viewer bound only to "local" cannot see cluster B's server -
	viewerName, viewerPW, viewerID := envInstance.CreateUser(t, admin, "viewer", "e2e-mc-viewer")
	t.Cleanup(func() {
		_, _, _ = admin.Delete("/users/" + viewerID)
	})
	viewer := envInstance.APIClient(t, viewerName, viewerPW)
	defer viewer.Close()

	// Sanity check first: the viewer's default-cluster read still works, so
	// a subsequent 403 below is provably about the cluster dimension and not
	// a broken viewer session.
	if resp, body, err := viewer.Get("/servers"); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("viewer GET /servers (default cluster): status=%v err=%v body=%s", resp, err, string(body))
	}

	resp, body, err = viewer.Get("/servers?cluster=" + clusterID)
	if err != nil {
		t.Fatalf("viewer GET /servers?cluster=%s: %v", clusterID, err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("viewer GET /servers?cluster=%s: status=%d want=%d body=%s",
			clusterID, resp.StatusCode, http.StatusForbidden, string(body))
	}

	// --- An unregistered cluster is a 400 for any caller, admin included ----
	resp, body, err = admin.Get("/servers?cluster=e2e-mc-does-not-exist")
	if err != nil {
		t.Fatalf("admin GET /servers?cluster=<unknown>: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("admin GET /servers?cluster=<unknown>: status=%d want=%d body=%s",
			resp.StatusCode, http.StatusBadRequest, string(body))
	}
}
