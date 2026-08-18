package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/ValgulNecron/gameplane/api/internal/auth"
	"github.com/ValgulNecron/gameplane/api/internal/db"
	"github.com/ValgulNecron/gameplane/api/internal/kube"
)

// wantShareNotFoundBody is the exact byte-for-byte body every "invalid"
// public share response must produce: unknown, expired, revoked, and
// canStart=false all collapse onto this single response so probing a
// token reveals nothing about why it failed.
var wantShareNotFoundBody = []byte(`{"error":"not found"}` + "\n")

// mountSharesRouter wires both the authenticated (owner) and public share
// routes onto one router, mirroring how api/cmd/main.go mounts them (in
// different middleware groups, but against the same reg/store).
func mountSharesRouter(reg *kube.Registry, store *db.Store) http.Handler {
	r := chi.NewRouter()
	MountShareLinks(r, reg, store)
	MountPublicShares(r, reg, store, auth.ShareLimiter)
	return r
}

// insertShareTestUser inserts a minimal user row and returns its ID, so
// created_by / the owner-id annotation can reference a real user.
func insertShareTestUser(t *testing.T, store *db.Store, username string) int64 {
	t.Helper()
	res, err := store.DB.ExecContext(t.Context(), `INSERT INTO users(username, role) VALUES (?, ?)`, username, "admin")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}
	return id
}

// newShareTestServer builds a GameServer unstructured object in the default
// namespace, owned by ownerID, with one public endpoint and a player count.
func newShareTestServer(name string, ownerID int64) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameServer",
		"metadata": map[string]any{
			"name":      name,
			"namespace": "gameplane-games",
			"annotations": map[string]any{
				"gameplane.local/owner-id": strconv.FormatInt(ownerID, 10),
			},
		},
		"status": map[string]any{
			"phase": "Running",
			"endpoints": []any{
				map[string]any{
					"name":     "game",
					"host":     "play.example.com",
					"port":     int64(25565),
					"private":  false,
					"protocol": "TCP",
				},
			},
			"agent": map[string]any{
				"playersOnline": int64(3),
			},
		},
	}}
}

// newShareTestServerPrivateOnly is like newShareTestServer but its only
// endpoint is a private/tailnet one, which must never be handed to an
// anonymous caller.
func newShareTestServerPrivateOnly(name string, ownerID int64) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameServer",
		"metadata": map[string]any{
			"name":      name,
			"namespace": "gameplane-games",
			"annotations": map[string]any{
				"gameplane.local/owner-id": strconv.FormatInt(ownerID, 10),
			},
		},
		"status": map[string]any{
			"phase": "Running",
			"endpoints": []any{
				map[string]any{
					"name":     "tailscale",
					"host":     "gameplane-srv.user.ts.net",
					"port":     int64(25565),
					"private":  true,
					"protocol": "TCP",
				},
			},
		},
	}}
}

// shareReq issues an HTTP request against h, optionally carrying an
// authenticated user in the context and a distinct RemoteAddr (so each
// test gets its own bucket in the shared, package-level auth.ShareLimiter).
func shareReq(t *testing.T, h http.Handler, method, path string, body any, user *auth.User, remoteAddr string) (int, []byte) {
	t.Helper()
	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		buf = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, buf)
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}
	if user != nil {
		req = req.WithContext(auth.WithUser(req.Context(), user))
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr.Code, rr.Body.Bytes()
}

// TestShareCreateReturnsTokenOnce verifies that the token is returned in the
// create response but never in list responses.
func TestShareCreateReturnsTokenOnce(t *testing.T) {
	store := newTestStore(t)
	ownerID := insertShareTestUser(t, store, "owner-create")
	reg := kube.NewRegistry("local")
	reg.Set("local", fakeKubeClient(newShareTestServer("srv-create", ownerID)))
	h := mountSharesRouter(reg, store)
	owner := &auth.User{ID: ownerID, Username: "owner-create", Role: "admin"}

	status, body := shareReq(t, h, "POST", "/servers/srv-create:shares", map[string]any{"canStart": false}, owner, "203.0.113.1:1")
	if status != http.StatusOK {
		t.Fatalf("create status = %d, want 200; body=%s", status, body)
	}
	var created map[string]any
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("unmarshal create response: %v; body=%s", err, body)
	}
	token, _ := created["token"].(string)
	if token == "" {
		t.Fatalf("create response missing non-empty token; body=%s", body)
	}

	status, body = shareReq(t, h, "GET", "/servers/srv-create:shares", nil, owner, "203.0.113.1:1")
	if status != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body=%s", status, body)
	}
	var listed []map[string]any
	if err := json.Unmarshal(body, &listed); err != nil {
		t.Fatalf("unmarshal list response: %v; body=%s", err, body)
	}
	if len(listed) != 1 {
		t.Fatalf("list length = %d, want 1; body=%s", len(listed), body)
	}
	for i, item := range listed {
		if v, ok := item["token"]; ok {
			t.Fatalf("list item %d carries a token field (%v); list must never include it", i, v)
		}
	}
}

// TestShareResolveValidToken verifies that a valid token resolves successfully
// and returns the server's public view, going through the real handler and
// cluster resolution (the path that a `reg.Get("")` bug would 404 on).
func TestShareResolveValidToken(t *testing.T) {
	store := newTestStore(t)
	ownerID := insertShareTestUser(t, store, "owner-resolve")
	reg := kube.NewRegistry("local")
	reg.Set("local", fakeKubeClient(newShareTestServer("srv-resolve", ownerID)))
	h := mountSharesRouter(reg, store)
	owner := &auth.User{ID: ownerID, Username: "owner-resolve", Role: "admin"}

	status, body := shareReq(t, h, "POST", "/servers/srv-resolve:shares", map[string]any{"canStart": false}, owner, "203.0.113.2:1")
	if status != http.StatusOK {
		t.Fatalf("create status = %d, want 200; body=%s", status, body)
	}
	var created map[string]any
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	token, _ := created["token"].(string)
	if token == "" {
		t.Fatalf("missing token in create response; body=%s", body)
	}

	status, body = shareReq(t, h, "GET", "/shares/"+token, nil, nil, "203.0.113.3:1")
	if status != http.StatusOK {
		t.Fatalf("resolve status = %d, want 200; body=%s", status, body)
	}
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal resolve response: %v; body=%s", err, body)
	}
	if resp["serverName"] != "srv-resolve" {
		t.Fatalf("serverName = %v, want srv-resolve; body=%s", resp["serverName"], body)
	}
	if resp["status"] != "Running" {
		t.Fatalf("status = %v, want Running; body=%s", resp["status"], body)
	}
	addr, ok := resp["address"].(map[string]any)
	if !ok {
		t.Fatalf("address missing or wrong type; body=%s", body)
	}
	if addr["host"] != "play.example.com" {
		t.Fatalf("address.host = %v, want play.example.com; body=%s", addr["host"], body)
	}
	if pf, ok := addr["port"].(float64); !ok || int32(pf) != 25565 {
		t.Fatalf("address.port = %v, want 25565; body=%s", addr["port"], body)
	}
	if pl, ok := resp["playersOnline"].(float64); !ok || int32(pl) != 3 {
		t.Fatalf("playersOnline = %v, want 3; body=%s", resp["playersOnline"], body)
	}
}

// TestShareResolvePrivateEndpointOmitted verifies, through the full public
// handler (not just the getPublicAddress helper directly), that a
// private/tailnet-only endpoint is never handed to an anonymous caller.
func TestShareResolvePrivateEndpointOmitted(t *testing.T) {
	store := newTestStore(t)
	ownerID := insertShareTestUser(t, store, "owner-private")
	reg := kube.NewRegistry("local")
	reg.Set("local", fakeKubeClient(newShareTestServerPrivateOnly("srv-private", ownerID)))
	h := mountSharesRouter(reg, store)
	owner := &auth.User{ID: ownerID, Username: "owner-private", Role: "admin"}

	_, body := shareReq(t, h, "POST", "/servers/srv-private:shares", map[string]any{"canStart": false}, owner, "203.0.113.4:1")
	var created map[string]any
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("unmarshal create response: %v; body=%s", err, body)
	}
	token, _ := created["token"].(string)
	if token == "" {
		t.Fatalf("missing token; body=%s", body)
	}

	status, body := shareReq(t, h, "GET", "/shares/"+token, nil, nil, "203.0.113.5:1")
	if status != http.StatusOK {
		t.Fatalf("resolve status = %d, want 200; body=%s", status, body)
	}
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, body)
	}
	if _, ok := resp["address"]; ok {
		t.Fatalf("address must be omitted for a private-only endpoint, got %v; body=%s", resp["address"], body)
	}
}

// TestShareResolveInvalidToken verifies that unknown, expired, and revoked
// tokens are indistinguishable: identical status code AND identical body.
func TestShareResolveInvalidToken(t *testing.T) {
	store := newTestStore(t)
	ownerID := insertShareTestUser(t, store, "owner-invalid")
	reg := kube.NewRegistry("local")
	reg.Set("local", fakeKubeClient(newShareTestServer("srv-invalid", ownerID)))
	h := mountSharesRouter(reg, store)

	ctx := context.Background()

	// Expired: mint a valid link, then rewrite its expiry into the past
	// (CreateShareLink itself refuses to mint an already-expired link).
	expiredToken, expiredLink, err := store.CreateShareLink(ctx, "gameplane-games", "srv-invalid", ownerID, false, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("create expired link: %v", err)
	}
	if _, err := store.DB.ExecContext(ctx, `UPDATE share_links SET expires_at = ? WHERE id = ?`,
		time.Now().UTC().Add(-time.Hour).Format(time.RFC3339), expiredLink.ID); err != nil {
		t.Fatalf("force-expire link: %v", err)
	}

	// Revoked: mint a valid link, then revoke it.
	revokedToken, revokedLink, err := store.CreateShareLink(ctx, "gameplane-games", "srv-invalid", ownerID, false, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("create link to revoke: %v", err)
	}
	if err := store.RevokeShareLink(ctx, revokedLink.ID); err != nil {
		t.Fatalf("revoke link: %v", err)
	}

	cases := map[string]string{
		"unknown": "this-token-was-never-issued-abc123",
		"expired": expiredToken,
		"revoked": revokedToken,
	}

	var firstStatus int
	var firstBody []byte
	first := true
	for name, token := range cases {
		status, body := shareReq(t, h, "GET", "/shares/"+token, nil, nil, "203.0.113.6:1")
		if status != http.StatusNotFound {
			t.Fatalf("%s: status = %d, want 404; body=%s", name, status, body)
		}
		if !bytes.Equal(body, wantShareNotFoundBody) {
			t.Fatalf("%s: body = %q, want %q", name, body, wantShareNotFoundBody)
		}
		if first {
			firstStatus, firstBody = status, body
			first = false
			continue
		}
		if status != firstStatus || !bytes.Equal(body, firstBody) {
			t.Fatalf("%s: response (%d, %q) differs from baseline (%d, %q)", name, status, body, firstStatus, firstBody)
		}
	}
}

// TestShareStartCanStartGate verifies that canStart=false returns the same
// 404 as an invalid token (not 403, not a distinguishable body), while
// canStart=true wakes the server by stamping the idle-wake annotation.
func TestShareStartCanStartGate(t *testing.T) {
	store := newTestStore(t)
	ownerID := insertShareTestUser(t, store, "owner-start")
	reg := kube.NewRegistry("local")
	k := fakeKubeClient(newShareTestServer("srv-start", ownerID))
	reg.Set("local", k)
	h := mountSharesRouter(reg, store)
	ctx := context.Background()

	// canStart=false must 404, identically to an invalid token.
	noStartToken, _, err := store.CreateShareLink(ctx, "gameplane-games", "srv-start", ownerID, false, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("create canStart=false link: %v", err)
	}
	status, body := shareReq(t, h, "POST", "/shares/"+noStartToken+"/start", nil, nil, "203.0.113.7:1")
	if status != http.StatusNotFound {
		t.Fatalf("canStart=false: status = %d, want 404; body=%s", status, body)
	}
	if !bytes.Equal(body, wantShareNotFoundBody) {
		t.Fatalf("canStart=false: body = %q, want %q (must be indistinguishable from an invalid token)", body, wantShareNotFoundBody)
	}

	// canStart=true must wake the server: 202 + idle-wake annotation stamped,
	// and the API must not touch spec.suspend (the operator owns that).
	startToken, _, err := store.CreateShareLink(ctx, "gameplane-games", "srv-start", ownerID, true, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("create canStart=true link: %v", err)
	}
	status, body = shareReq(t, h, "POST", "/shares/"+startToken+"/start", nil, nil, "203.0.113.8:1")
	if status != http.StatusAccepted {
		t.Fatalf("canStart=true: status = %d, want 202; body=%s", status, body)
	}

	obj, err := k.Dynamic.Resource(kube.GVRs["servers"]).Namespace("gameplane-games").Get(ctx, "srv-start", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("fetch server after start: %v", err)
	}
	ann := obj.GetAnnotations()
	if ann[idleWakeRequestedAnnotation] == "" {
		t.Fatalf("expected %s annotation to be stamped, got annotations %+v", idleWakeRequestedAnnotation, ann)
	}
	if _, found, _ := unstructured.NestedString(obj.Object, "spec", "suspend"); found {
		t.Fatalf("API must not write spec.suspend directly; found value %v", obj.Object["spec"])
	}
}

// TestSharePublicPayloadSafe verifies that the public response contains no
// forbidden fields (no namespace, cluster, version, owner, etc.).
func TestSharePublicPayloadSafe(t *testing.T) {
	t.Parallel()
	// Create a mock GameServer object with all possible fields.
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "gameplane.local/v1alpha1",
			"kind":       "GameServer",
			"metadata": map[string]interface{}{
				"name":      "test-server",
				"namespace": "gameplane-games",
				"annotations": map[string]interface{}{
					"gameplane.local/owner-id": "42",
				},
			},
			"spec": map[string]interface{}{
				"templateRef": map[string]interface{}{
					"name": "minecraft-java",
				},
			},
			"status": map[string]interface{}{
				"phase": "Running",
				"endpoints": []interface{}{
					map[string]interface{}{
						"name":     "game",
						"host":     "192.0.2.1",
						"port":     float64(25565), // JSON unmarshals numbers as float64
						"private":  false,
						"protocol": "TCP",
					},
				},
				"agent": map[string]interface{}{
					"playersOnline": float64(5),
					"playersMax":    float64(20),
					"gameVersion":   "1.20.1",
				},
			},
		},
	}

	// Extract the public response fields.
	status := getPhase(obj)
	address := getPublicAddress(obj)
	players := getPlayersOnline(obj)

	// Verify status is present and safe.
	if status != "Running" {
		t.Fatalf("phase: want Running, got %s", status)
	}

	// Verify address is present and safe.
	if address == nil {
		t.Fatal("address must not be nil")
	}
	if address.Host != "192.0.2.1" || address.Port != 25565 {
		t.Fatalf("address: got %+v, want host=192.0.2.1 port=25565", address)
	}

	// Verify players count is present.
	if players == nil || *players != 5 {
		t.Fatalf("players: got %v, want 5", players)
	}

	// Marshal to JSON and verify no forbidden keys are present.
	resp := sharePublicResp{
		ServerName:    "test-server",
		Status:        status,
		Address:       address,
		PlayersOnline: players,
	}
	body, _ := json.Marshal(resp)
	var m map[string]interface{}
	json.Unmarshal(body, &m)

	// Forbidden fields that must not appear in the response.
	forbidden := []string{"namespace", "cluster", "version", "owner", "template", "gameVersion", "playersMax"}
	for _, key := range forbidden {
		if _, ok := m[key]; ok {
			t.Fatalf("forbidden field %q found in response", key)
		}
	}

	// The response should only have the safe fields.
	expectedKeys := map[string]bool{
		"serverName":    true,
		"status":        true,
		"address":       true,
		"playersOnline": true,
	}
	for key := range m {
		if !expectedKeys[key] {
			t.Fatalf("unexpected key %q in response", key)
		}
	}
}

// TestSharePrivateEndpointNotReturned verifies that private/tailnet-only endpoints
// are never returned to anonymous callers.
func TestSharePrivateEndpointNotReturned(t *testing.T) {
	t.Parallel()
	// Create a mock server with only private endpoints.
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"phase": "Running",
				"endpoints": []interface{}{
					map[string]interface{}{
						"name":     "tailscale",
						"host":     "gameplane-srv.user.ts.net",
						"port":     int64(25565),
						"private":  true,
						"protocol": "TCP",
					},
				},
			},
		},
	}

	// getPublicAddress should return nil because the only endpoint is private.
	address := getPublicAddress(obj)
	if address != nil {
		t.Fatalf("expected nil address for private-only endpoints, got %+v", address)
	}

	// Now add a public endpoint and verify it's returned.
	obj.Object["status"].(map[string]interface{})["endpoints"] = []interface{}{
		map[string]interface{}{
			"name":     "tailscale",
			"host":     "gameplane-srv.user.ts.net",
			"port":     int64(25565),
			"private":  true,
			"protocol": "TCP",
		},
		map[string]interface{}{
			"name":           "tunnel",
			"host":           "play.example.com",
			"port":           int64(25565),
			"private":        false,
			"protocol":       "TCP",
			"tunnelProvider": "frp",
		},
	}

	address = getPublicAddress(obj)
	if address == nil {
		t.Fatal("expected address for mixed endpoints, got nil")
	}
	if address.Host != "play.example.com" {
		t.Fatalf("expected public endpoint, got %s", address.Host)
	}
}

// TestShareRevokeOwnerOnly verifies that only the owner can revoke a share
// link, and that revocation actually takes effect: the token stops
// resolving afterwards.
func TestShareRevokeOwnerOnly(t *testing.T) {
	store := newTestStore(t)
	ownerID := insertShareTestUser(t, store, "owner-revoke")
	strangerID := insertShareTestUser(t, store, "stranger-revoke")
	reg := kube.NewRegistry("local")
	reg.Set("local", fakeKubeClient(newShareTestServer("srv-revoke", ownerID)))
	h := mountSharesRouter(reg, store)
	owner := &auth.User{ID: ownerID, Username: "owner-revoke", Role: "admin"}
	stranger := &auth.User{ID: strangerID, Username: "stranger-revoke", Role: "admin"}

	status, body := shareReq(t, h, "POST", "/servers/srv-revoke:shares", map[string]any{"canStart": false}, owner, "203.0.113.9:1")
	if status != http.StatusOK {
		t.Fatalf("create status = %d, want 200; body=%s", status, body)
	}
	var created map[string]any
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	token, _ := created["token"].(string)
	id, _ := created["id"].(string)
	if token == "" || id == "" {
		t.Fatalf("create response missing token/id; body=%s", body)
	}

	// A non-owner must not be able to revoke.
	status, _ = shareReq(t, h, "DELETE", "/servers/srv-revoke/shares/"+id, nil, stranger, "203.0.113.10:1")
	if status != http.StatusForbidden {
		t.Fatalf("non-owner revoke status = %d, want 403", status)
	}

	// The link must still resolve (revoke by a non-owner had no effect).
	status, _ = shareReq(t, h, "GET", "/shares/"+token, nil, nil, "203.0.113.11:1")
	if status != http.StatusOK {
		t.Fatalf("resolve after failed revoke: status = %d, want 200", status)
	}

	// The owner can revoke.
	status, body = shareReq(t, h, "DELETE", "/servers/srv-revoke/shares/"+id, nil, owner, "203.0.113.12:1")
	if status != http.StatusNoContent {
		t.Fatalf("owner revoke status = %d, want 204; body=%s", status, body)
	}

	// The token must stop resolving, with the same 404 body as any other
	// invalid token.
	status, body = shareReq(t, h, "GET", "/shares/"+token, nil, nil, "203.0.113.13:1")
	if status != http.StatusNotFound {
		t.Fatalf("resolve after revoke: status = %d, want 404; body=%s", status, body)
	}
	if !bytes.Equal(body, wantShareNotFoundBody) {
		t.Fatalf("resolve after revoke: body = %q, want %q", body, wantShareNotFoundBody)
	}
}
