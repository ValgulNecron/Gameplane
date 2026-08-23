package handlers

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ValgulNecron/gameplane/api/internal/audit"
	"github.com/ValgulNecron/gameplane/api/internal/httperr"
	"github.com/ValgulNecron/gameplane/api/internal/kube"
)

// CaptureConfig holds cluster-wide capture defaults and limits.
type CaptureConfig struct {
	DefaultRetentionSeconds int64
	MaxRetentionSeconds     int64
	DefaultMaxDurationSecs  int
	DefaultMaxSizeBytes     int64
}

// MountCapture wires capture endpoints: start, stop, list, get, and download.
// cfg contains cluster-wide capture defaults and limits.
// The mTLS cert paths are used to authenticate to the capture sidecar's :9091 endpoint.
func MountCapture(r chi.Router, reg *kube.Registry, auditor *audit.Auditor, cfg CaptureConfig, agentCABundle, agentClientCert, agentClientKey string) {
	var tlsClient *http.Client
	if agentCABundle != "" && agentClientCert != "" && agentClientKey != "" {
		tlsClient = buildAgentHTTPClient(agentCABundle, agentClientCert, agentClientKey)
	}

	h := &captureHandler{
		reg:       reg,
		auditor:   auditor,
		cfg:       cfg,
		tlsClient: tlsClient,
	}
	r.Post("/servers/{name}:capture-start", h.captureStart)
	r.Post("/servers/{name}:capture-stop", h.captureStop)
	r.Get("/servers/{name}:captures", h.captureList)
	r.Get("/servers/{name}:capture", h.captureGet)
	r.Get("/servers/{name}:capture-file", h.captureDownload)
}

type captureHandler struct {
	reg       *kube.Registry
	auditor   *audit.Auditor
	cfg       CaptureConfig
	tlsClient *http.Client
}

// errCaptureNotFound is a sentinel distinguishing "no such capture, or it
// belongs to a different server" from a real Kubernetes API failure, so
// callers can tell a 404 apart from a 500 without string-matching errors.
var errCaptureNotFound = errors.New("capture not found")

type captureStartReq struct {
	Filter                string `json:"filter,omitempty"`
	MaxDurationSeconds    int    `json:"maxDurationSeconds"`
	MaxSizeBytes          int64  `json:"maxSizeBytes"`
	TTLSecondsAfterFinish int64  `json:"ttlSecondsAfterFinished,omitempty"`
}

type captureStartResp struct {
	CaptureID             string `json:"captureId"`
	Phase                 string `json:"phase"`
	ServerName            string `json:"serverName"`
	Filter                string `json:"filter"`
	MaxDurationSeconds    int    `json:"maxDurationSeconds"`
	MaxSizeBytes          int64  `json:"maxSizeBytes"`
	TTLSecondsAfterFinish int64  `json:"ttlSecondsAfterFinished"`
	CreatedAt             string `json:"createdAt"`
	StartedAt             string `json:"startedAt"`
	CompletedAt           string `json:"completedAt"`
	BytesWritten          int64  `json:"bytesWritten"`
	PacketsWritten        int64  `json:"packetsWritten"`
}

// captureStart creates a new NetworkCapture CR and returns it.
// POST /servers/{name}:capture-start
func (h *captureHandler) captureStart(w http.ResponseWriter, req *http.Request) {
	k, ok := resolveCluster(w, req, h.reg)
	if !ok {
		return
	}
	name := chi.URLParam(req, "name")
	ns, ok := resolveNS(w, req)
	if !ok {
		return
	}
	auditPath := fmt.Sprintf("/servers/%s:capture-start", name)

	var body captureStartReq
	if err := json.NewDecoder(io.LimitReader(req.Body, 1<<16)).Decode(&body); err != nil {
		httperr.WriteCode(w, req, http.StatusBadRequest, errors.New("invalid request body"))
		return
	}

	// Validate duration range (FR-002). This ceiling is fixed by the
	// contract (rest-api.md), not by the cluster's configurable default —
	// the default only informs what a client might pre-fill.
	if body.MaxDurationSeconds < 1 || body.MaxDurationSeconds > 3600 {
		if !h.auditWriteOrFail(w, req, http.MethodPost, auditPath, name, "invalid_duration", http.StatusBadRequest) {
			return
		}
		httperr.WriteCode(w, req, http.StatusBadRequest, errors.New("maxDurationSeconds must be 1..3600"))
		return
	}

	// Validate size range (FR-002), capped by the cluster-configured max.
	if body.MaxSizeBytes < 1 || (h.cfg.DefaultMaxSizeBytes > 0 && body.MaxSizeBytes > h.cfg.DefaultMaxSizeBytes) {
		if !h.auditWriteOrFail(w, req, http.MethodPost, auditPath, name, "invalid_size", http.StatusBadRequest) {
			return
		}
		httperr.WriteCode(w, req, http.StatusBadRequest, fmt.Errorf("maxSizeBytes must be 1..%d", h.cfg.DefaultMaxSizeBytes))
		return
	}

	// Validate the filter expression (FR-003) before creating any CRD.
	if err := validateFilter(body.Filter); err != nil {
		if !h.auditWriteOrFail(w, req, http.MethodPost, auditPath, name, "invalid_filter", http.StatusBadRequest) {
			return
		}
		httperr.WriteCode(w, req, http.StatusBadRequest, err)
		return
	}

	// Check the server exists and has capture enabled.
	gs, err := k.GetGameServer(req.Context(), ns, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if !h.auditWriteOrFail(w, req, http.MethodPost, auditPath, name, "server_not_found", http.StatusNotFound) {
				return
			}
			httperr.WriteCode(w, req, http.StatusNotFound, errors.New("not found"))
		} else {
			if !h.auditWriteOrFail(w, req, http.MethodPost, auditPath, name, "error", http.StatusInternalServerError) {
				return
			}
			httperr.Write(w, req, err)
		}
		return
	}

	if !isCaptureEnabled(gs) {
		if !h.auditWriteOrFail(w, req, http.MethodPost, auditPath, name, "capture_not_enabled", http.StatusBadRequest) {
			return
		}
		httperr.WriteCode(w, req, http.StatusBadRequest, errors.New("capture is not enabled on this server"))
		return
	}

	// Check no other capture is already Pending/Running (FR-012).
	active, err := h.hasActiveCapture(req.Context(), k, gs, ns, name)
	if err != nil {
		if !h.auditWriteOrFail(w, req, http.MethodPost, auditPath, name, "error", http.StatusInternalServerError) {
			return
		}
		httperr.Write(w, req, err)
		return
	}
	if active {
		if !h.auditWriteOrFail(w, req, http.MethodPost, auditPath, name, "capture_in_progress", http.StatusConflict) {
			return
		}
		httperr.WriteCode(w, req, http.StatusConflict, errors.New("capture already in progress on this server"))
		return
	}

	// Validate and clamp TTL to the cluster max — never trust the client's
	// requested value directly.
	ttl := body.TTLSecondsAfterFinish
	if ttl <= 0 {
		ttl = h.cfg.DefaultRetentionSeconds
	}
	if ttl > h.cfg.MaxRetentionSeconds {
		if !h.auditWriteOrFail(w, req, http.MethodPost, auditPath, name, "ttl_exceeded", http.StatusBadRequest) {
			return
		}
		httperr.WriteCode(w, req, http.StatusBadRequest,
			fmt.Errorf("requested retention %ds exceeds cluster maximum %ds", ttl, h.cfg.MaxRetentionSeconds))
		return
	}

	captureID := generateCaptureID()
	var filterPtr *string
	if body.Filter != "" {
		filterPtr = &body.Filter
	}
	maxDuration := &metav1.Duration{Duration: time.Duration(body.MaxDurationSeconds) * time.Second}
	maxSize := resource.NewQuantity(body.MaxSizeBytes, resource.BinarySI)
	ttl32 := int32(ttl)

	created, err := k.CreateNetworkCapture(req.Context(), ns, captureID, name, gs.UID, filterPtr, maxDuration, maxSize, &ttl32)
	if err != nil {
		if !h.auditWriteOrFail(w, req, http.MethodPost, auditPath, name, "create_failed", http.StatusInternalServerError) {
			return
		}
		httperr.Write(w, req, err)
		return
	}

	resp := captureStartResp{
		CaptureID:             created.Name,
		Phase:                 string(created.Status.Phase),
		ServerName:            name,
		Filter:                body.Filter,
		MaxDurationSeconds:    body.MaxDurationSeconds,
		MaxSizeBytes:          body.MaxSizeBytes,
		TTLSecondsAfterFinish: ttl,
		CreatedAt:             formatTime(created.CreationTimestamp.Time),
	}

	// Audit the successful creation before writing the response.
	if !h.auditWriteOrFail(w, req, http.MethodPost, auditPath, fmt.Sprintf("%s:%s", name, created.Name), "", http.StatusAccepted) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(resp)
}

type captureStopReq struct {
	CaptureID string `json:"captureId"`
}

type captureStopResp struct {
	CaptureID      string `json:"captureId"`
	Phase          string `json:"phase"`
	ServerName     string `json:"serverName"`
	Filter         string `json:"filter"`
	CreatedAt      string `json:"createdAt"`
	StartedAt      string `json:"startedAt"`
	CompletedAt    string `json:"completedAt"`
	StoppingReason string `json:"stoppingReason,omitempty"`
	BytesWritten   int64  `json:"bytesWritten"`
	PacketsWritten int64  `json:"packetsWritten"`
}

// captureStop stops a running capture.
// POST /servers/{name}:capture-stop
func (h *captureHandler) captureStop(w http.ResponseWriter, req *http.Request) {
	k, ok := resolveCluster(w, req, h.reg)
	if !ok {
		return
	}
	name := chi.URLParam(req, "name")
	ns, ok := resolveNS(w, req)
	if !ok {
		return
	}
	auditPath := fmt.Sprintf("/servers/%s:capture-stop", name)

	var body captureStopReq
	if err := json.NewDecoder(io.LimitReader(req.Body, 1<<16)).Decode(&body); err != nil {
		httperr.WriteCode(w, req, http.StatusBadRequest, errors.New("invalid request body"))
		return
	}

	if body.CaptureID == "" {
		if !h.auditWriteOrFail(w, req, http.MethodPost, auditPath, name, "missing_id", http.StatusBadRequest) {
			return
		}
		httperr.WriteCode(w, req, http.StatusBadRequest, errors.New("captureId is required"))
		return
	}
	target := fmt.Sprintf("%s:%s", name, body.CaptureID)

	nc, err := h.resolveCapture(req.Context(), k, ns, name, body.CaptureID)
	if err != nil {
		if errors.Is(err, errCaptureNotFound) {
			if !h.auditWriteOrFail(w, req, http.MethodPost, auditPath, target, "not_found", http.StatusNotFound) {
				return
			}
			httperr.WriteCode(w, req, http.StatusNotFound, fmt.Errorf("capture '%s' not found", body.CaptureID))
		} else {
			if !h.auditWriteOrFail(w, req, http.MethodPost, auditPath, target, "error", http.StatusInternalServerError) {
				return
			}
			httperr.Write(w, req, err)
		}
		return
	}

	if nc.Status.Phase != gameplanev1alpha1.CapturePhasePending && nc.Status.Phase != gameplanev1alpha1.CapturePhaseRunning {
		if !h.auditWriteOrFail(w, req, http.MethodPost, auditPath, target, "not_running", http.StatusConflict) {
			return
		}
		httperr.WriteCode(w, req, http.StatusConflict, errors.New("capture is not running"))
		return
	}

	stopped, err := k.StopNetworkCapture(req.Context(), ns, body.CaptureID)
	if err != nil {
		if !h.auditWriteOrFail(w, req, http.MethodPost, auditPath, target, "stop_failed", http.StatusInternalServerError) {
			return
		}
		httperr.Write(w, req, err)
		return
	}

	resp := captureStopResp{
		CaptureID:      stopped.Name,
		Phase:          string(stopped.Status.Phase),
		ServerName:     name,
		Filter:         filterString(stopped.Spec.Filter),
		CreatedAt:      formatTime(stopped.CreationTimestamp.Time),
		StartedAt:      formatOptionalTime(stopped.Status.StartTime),
		CompletedAt:    formatOptionalTime(stopped.Status.CompletionTime),
		StoppingReason: "user_requested",
		BytesWritten:   quantityValue(stopped.Status.BytesWritten),
		PacketsWritten: stopped.Status.PacketsWritten,
	}

	if !h.auditWriteOrFail(w, req, http.MethodPost, auditPath, target, "", http.StatusOK) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

type captureListResp struct {
	Captures []captureItem `json:"captures"`
	Total    int           `json:"total"`
	Limit    int           `json:"limit"`
	Offset   int           `json:"offset"`
}

type captureItem struct {
	CaptureID      string `json:"captureId"`
	Phase          string `json:"phase"`
	ServerName     string `json:"serverName"`
	Filter         string `json:"filter"`
	CreatedAt      string `json:"createdAt"`
	StartedAt      string `json:"startedAt"`
	CompletedAt    string `json:"completedAt"`
	BytesWritten   int64  `json:"bytesWritten"`
	PacketsWritten int64  `json:"packetsWritten"`
	ExpiresAt      string `json:"expiresAt,omitempty"`
}

// captureList lists captures for a server.
// GET /servers/{name}:captures
func (h *captureHandler) captureList(w http.ResponseWriter, req *http.Request) {
	k, ok := resolveCluster(w, req, h.reg)
	if !ok {
		return
	}
	name := chi.URLParam(req, "name")
	ns, ok := resolveNS(w, req)
	if !ok {
		return
	}
	auditPath := fmt.Sprintf("/servers/%s:captures", name)

	if _, err := k.GetGameServer(req.Context(), ns, name); err != nil {
		if apierrors.IsNotFound(err) {
			if !h.auditWriteOrFail(w, req, http.MethodGet, auditPath, name, "server_not_found", http.StatusNotFound) {
				return
			}
			httperr.WriteCode(w, req, http.StatusNotFound, errors.New("not found"))
		} else {
			if !h.auditWriteOrFail(w, req, http.MethodGet, auditPath, name, "error", http.StatusInternalServerError) {
				return
			}
			httperr.Write(w, req, err)
		}
		return
	}

	captures, err := k.ListNetworkCaptures(req.Context(), ns, name)
	if err != nil {
		if !h.auditWriteOrFail(w, req, http.MethodGet, auditPath, name, "list_failed", http.StatusInternalServerError) {
			return
		}
		httperr.Write(w, req, err)
		return
	}

	// Audit the list before writing the response (FR-006 / the GET-audit
	// gap: shouldLog skips GET, so this call site must exist).
	if !h.auditWriteOrFail(w, req, http.MethodGet, auditPath, name, "", http.StatusOK) {
		return
	}

	items := make([]captureItem, 0, len(captures))
	for _, c := range captures {
		if c.Status.Phase == gameplanev1alpha1.CapturePhaseExpired {
			continue
		}
		items = append(items, captureItem{
			CaptureID:      c.Name,
			Phase:          string(c.Status.Phase),
			ServerName:     name,
			Filter:         filterString(c.Spec.Filter),
			CreatedAt:      formatTime(c.CreationTimestamp.Time),
			StartedAt:      formatOptionalTime(c.Status.StartTime),
			CompletedAt:    formatOptionalTime(c.Status.CompletionTime),
			BytesWritten:   quantityValue(c.Status.BytesWritten),
			PacketsWritten: c.Status.PacketsWritten,
			ExpiresAt:      h.expiresAt(c),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(captureListResp{
		Captures: items,
		Total:    len(items),
		Limit:    100,
		Offset:   0,
	})
}

type captureGetResp struct {
	CaptureID          string `json:"captureId"`
	Phase              string `json:"phase"`
	ServerName         string `json:"serverName"`
	Filter             string `json:"filter"`
	MaxDurationSeconds int    `json:"maxDurationSeconds"`
	MaxSizeBytes       int64  `json:"maxSizeBytes"`
	CreatedAt          string `json:"createdAt"`
	StartedAt          string `json:"startedAt"`
	CompletedAt        string `json:"completedAt"`
	BytesWritten       int64  `json:"bytesWritten"`
	PacketsWritten     int64  `json:"packetsWritten"`
	ExpiresAt          string `json:"expiresAt,omitempty"`
}

// captureGet gets a single capture by ID.
// GET /servers/{name}:capture?id={id}
func (h *captureHandler) captureGet(w http.ResponseWriter, req *http.Request) {
	k, ok := resolveCluster(w, req, h.reg)
	if !ok {
		return
	}
	name := chi.URLParam(req, "name")
	ns, ok := resolveNS(w, req)
	if !ok {
		return
	}
	captureID := req.URL.Query().Get("id")
	auditPath := fmt.Sprintf("/servers/%s:capture", name)
	target := fmt.Sprintf("%s:%s", name, captureID)

	if captureID == "" {
		if !h.auditWriteOrFail(w, req, http.MethodGet, auditPath, target, "missing_id", http.StatusBadRequest) {
			return
		}
		httperr.WriteCode(w, req, http.StatusBadRequest, errors.New("id is required"))
		return
	}

	if _, err := k.GetGameServer(req.Context(), ns, name); err != nil {
		if apierrors.IsNotFound(err) {
			if !h.auditWriteOrFail(w, req, http.MethodGet, auditPath, target, "server_not_found", http.StatusNotFound) {
				return
			}
			httperr.WriteCode(w, req, http.StatusNotFound, errors.New("not found"))
		} else {
			if !h.auditWriteOrFail(w, req, http.MethodGet, auditPath, target, "error", http.StatusInternalServerError) {
				return
			}
			httperr.Write(w, req, err)
		}
		return
	}

	nc, err := h.resolveCapture(req.Context(), k, ns, name, captureID)
	if err != nil {
		if errors.Is(err, errCaptureNotFound) {
			if !h.auditWriteOrFail(w, req, http.MethodGet, auditPath, target, "not_found", http.StatusNotFound) {
				return
			}
			httperr.WriteCode(w, req, http.StatusNotFound, fmt.Errorf("capture '%s' not found or has expired", captureID))
		} else {
			if !h.auditWriteOrFail(w, req, http.MethodGet, auditPath, target, "error", http.StatusInternalServerError) {
				return
			}
			httperr.Write(w, req, err)
		}
		return
	}

	if nc.Status.Phase == gameplanev1alpha1.CapturePhaseExpired {
		if !h.auditWriteOrFail(w, req, http.MethodGet, auditPath, target, "expired", http.StatusNotFound) {
			return
		}
		httperr.WriteCode(w, req, http.StatusNotFound, fmt.Errorf("capture '%s' not found or has expired", captureID))
		return
	}

	if !h.auditWriteOrFail(w, req, http.MethodGet, auditPath, target, "", http.StatusOK) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(captureGetResp{
		CaptureID:          nc.Name,
		Phase:              string(nc.Status.Phase),
		ServerName:         name,
		Filter:             filterString(nc.Spec.Filter),
		MaxDurationSeconds: maxDurationSecondsOf(nc.Spec.MaxDuration),
		MaxSizeBytes:       quantityValue(nc.Spec.MaxSize),
		CreatedAt:          formatTime(nc.CreationTimestamp.Time),
		StartedAt:          formatOptionalTime(nc.Status.StartTime),
		CompletedAt:        formatOptionalTime(nc.Status.CompletionTime),
		BytesWritten:       quantityValue(nc.Status.BytesWritten),
		PacketsWritten:     nc.Status.PacketsWritten,
		ExpiresAt:          h.expiresAt(*nc),
	})
}

// captureDownload downloads the capture file from the sidecar.
// GET /servers/{name}:capture-file?id={id}
func (h *captureHandler) captureDownload(w http.ResponseWriter, req *http.Request) {
	k, ok := resolveCluster(w, req, h.reg)
	if !ok {
		return
	}
	name := chi.URLParam(req, "name")
	ns, ok := resolveNS(w, req)
	if !ok {
		return
	}
	captureID := req.URL.Query().Get("id")
	auditPath := fmt.Sprintf("/servers/%s:capture-file", name)
	target := fmt.Sprintf("%s:%s", name, captureID)

	if captureID == "" {
		if !h.auditWriteOrFail(w, req, http.MethodGet, auditPath, target, "missing_id", http.StatusBadRequest) {
			return
		}
		httperr.WriteCode(w, req, http.StatusBadRequest, errors.New("id is required"))
		return
	}

	gs, err := k.GetGameServer(req.Context(), ns, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if !h.auditWriteOrFail(w, req, http.MethodGet, auditPath, target, "server_not_found", http.StatusNotFound) {
				return
			}
			httperr.WriteCode(w, req, http.StatusNotFound, errors.New("not found"))
		} else {
			if !h.auditWriteOrFail(w, req, http.MethodGet, auditPath, target, "error", http.StatusInternalServerError) {
				return
			}
			httperr.Write(w, req, err)
		}
		return
	}

	// The upstream host is built from the resolved GameServer object's own
	// identity (gs.Name/gs.Namespace, populated from the Kubernetes API
	// response), not by re-reading the raw request values a second time —
	// see newValidatedHost below for why this matters for gosec's SSRF taint
	// analysis (G704).
	host, ok := newValidatedHost(gs.Name, gs.Namespace)
	if !ok {
		if !h.auditWriteOrFail(w, req, http.MethodGet, auditPath, target, "invalid_host", http.StatusBadRequest) {
			return
		}
		httperr.WriteCode(w, req, http.StatusBadRequest, errors.New("invalid namespace or pod name"))
		return
	}

	nc, err := h.resolveCapture(req.Context(), k, ns, name, captureID)
	if err != nil {
		if errors.Is(err, errCaptureNotFound) {
			if !h.auditWriteOrFail(w, req, http.MethodGet, auditPath, target, "not_found", http.StatusNotFound) {
				return
			}
			httperr.WriteCode(w, req, http.StatusNotFound, fmt.Errorf("capture '%s' not found or has expired", captureID))
		} else {
			if !h.auditWriteOrFail(w, req, http.MethodGet, auditPath, target, "error", http.StatusInternalServerError) {
				return
			}
			httperr.Write(w, req, err)
		}
		return
	}

	if nc.Status.Phase != gameplanev1alpha1.CapturePhaseCompleted {
		if nc.Status.Phase == gameplanev1alpha1.CapturePhaseExpired {
			if !h.auditWriteOrFail(w, req, http.MethodGet, auditPath, target, "expired", http.StatusNotFound) {
				return
			}
			httperr.WriteCode(w, req, http.StatusNotFound, fmt.Errorf("capture '%s' not found or has expired", captureID))
		} else {
			if !h.auditWriteOrFail(w, req, http.MethodGet, auditPath, target, "not_completed", http.StatusConflict) {
				return
			}
			httperr.WriteCode(w, req, http.StatusConflict, errors.New("capture is still running or has failed"))
		}
		return
	}

	// FR-006: the audit write happens BEFORE any response byte (including
	// WriteHeader) reaches the client — a failed audit write aborts the
	// download outright rather than letting an unaudited file stream
	// through to the caller. This first row necessarily records intent
	// (StatusOK), not the outcome: the sidecar hasn't been contacted yet.
	if !h.auditWriteOrFail(w, req, http.MethodGet, auditPath, target, "", http.StatusOK) {
		return
	}

	// Stream from sidecar over mTLS. If the proxy did not actually succeed,
	// correct the record above with a second row carrying the real status —
	// the response is already committed by then, so this can only append to
	// the trail, not gate it (that gate already happened above, per FR-006).
	status := h.proxyFileDownload(w, req, host, captureID)
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		if err := h.auditor.WriteSync(req.Context(), http.MethodGet, auditPath, target, "download_failed", status); err != nil {
			slog.Error("capture download: corrective audit write failed", "capture", target, "status", status, "err", err)
		}
	}
}

// validatedHost is an in-cluster "host:port" authority that can only be
// produced by newValidatedHost. Threading this type through the URL
// construction below — instead of the raw namespace/name strings — makes
// "already validated as a DNS-1123 label" a fact carried by the value's
// type, not just something the reader has to trust happened earlier in the
// function.
type validatedHost string

// newValidatedHost builds the capture sidecar's in-cluster Service DNS name
// (<gs>-agent.<ns>.svc.cluster.local:9091), or reports ok=false if either
// name or ns is not a valid DNS-1123 label. Both must be validated before
// any URL is built from them, to prevent SSRF injection.
func newValidatedHost(name, ns string) (host validatedHost, ok bool) {
	if !isDNS1123Label(name) || !isDNS1123Label(ns) {
		return "", false
	}
	return validatedHost(fmt.Sprintf("%s-agent.%s.svc.cluster.local:9091", name, ns)), true
}

// proxyFileDownload streams the capture file from the sidecar's :9091
// endpoint. host must come from newValidatedHost — the DNS-1123 checks are
// not repeated here, since the type itself is the evidence they already ran.
// It returns the status actually written to the client, so the caller can
// correct the FR-006 audit row (written before the proxy ran, and so
// necessarily optimistic) with what really happened.
func (h *captureHandler) proxyFileDownload(w http.ResponseWriter, req *http.Request, host validatedHost, captureID string) int {
	if h.tlsClient == nil {
		httperr.WriteCode(w, req, http.StatusServiceUnavailable, errors.New("agent mTLS not configured"))
		return http.StatusServiceUnavailable
	}

	// Construct URL using url.URL (never string concatenation) from the
	// pre-validated host.
	u := &url.URL{
		Scheme: "https",
		Host:   string(host),
		Path:   fmt.Sprintf("/captures/%s/file", captureID),
	}

	// Build request to sidecar, propagating the caller's context so a
	// client disconnect cancels the upstream fetch.
	upReq, err := http.NewRequestWithContext(req.Context(), http.MethodGet, u.String(), nil)
	if err != nil {
		httperr.Write(w, req, err)
		return http.StatusInternalServerError
	}

	// Forward only headers the sidecar actually consumes — never the
	// caller's Cookie/Authorization/CSRF material.
	copyProxyHeaders(upReq.Header, req.Header)

	resp, err := h.tlsClient.Do(upReq)
	if err != nil {
		return writeUpstreamError(w, req, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// A non-2xx from the sidecar is an error response (plain text, e.g.
	// "capture not found"), not a pcap payload — labeling it with the
	// download's Content-Type/Content-Disposition would make a browser
	// save an error message as "capture-<id>.pcapng". Route it through
	// httperr like any other handler error instead, and never touch the
	// download-specific headers on this path.
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		httperr.WriteCode(w, req, resp.StatusCode, fmt.Errorf("capture sidecar returned %s", http.StatusText(resp.StatusCode)))
		return resp.StatusCode
	}

	// Copy response headers (with denylist), then the fixed
	// download-specific headers, then stream the body straight through —
	// never buffer the whole file in memory.
	copyResponseHeaders(w.Header(), resp.Header)
	w.Header().Set("Content-Type", "application/vnd.tcpdump.pcap")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"capture-%s.pcapng\"", captureID))

	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
	return resp.StatusCode
}

// writeUpstreamError maps transport-level failures to gateway statuses and
// returns the status it wrote.
func writeUpstreamError(w http.ResponseWriter, req *http.Request, err error) int {
	status := http.StatusBadGateway
	var nerr net.Error
	if errors.As(err, &nerr) && nerr.Timeout() {
		status = http.StatusGatewayTimeout
	}
	httperr.WriteCode(w, req, status, errors.New("capture sidecar is unreachable"))
	return status
}

// auditWriteOrFail records an audit event and reports whether the caller
// may go on to write its own response. Per FR-006 ("audit events MUST be
// written before the operation response is returned"), a failed audit
// write is not survivable here: this writes a generic 500 itself and
// returns false, and every call site must return immediately without
// writing anything else.
func (h *captureHandler) auditWriteOrFail(w http.ResponseWriter, req *http.Request, method, path, target, reason string, status int) bool {
	if err := h.auditor.WriteSync(req.Context(), method, path, target, reason, status); err != nil {
		httperr.WriteCode(w, req, http.StatusInternalServerError, errors.New("audit write failed"))
		return false
	}
	return true
}

// buildAgentHTTPClient constructs an HTTP client configured with mTLS for agent communication.
// It follows the same pattern as ws/dialer.go's agentTLSConfig, loading the CA bundle and
// client cert/key from files.
func buildAgentHTTPClient(caBundle, clientCert, clientKey string) *http.Client {
	tlsCfg := &tls.Config{}

	// Load and configure CA bundle for server verification
	if caBundle != "" {
		caCert, err := os.ReadFile(caBundle)
		if err != nil {
			slog.Error("failed to read CA bundle", "path", caBundle, "err", err)
			return nil
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			slog.Error("failed to parse CA bundle", "path", caBundle)
			return nil
		}
		tlsCfg.RootCAs = caCertPool
	}

	// Load client certificate and key for mTLS
	if clientCert != "" && clientKey != "" {
		cert, err := tls.LoadX509KeyPair(clientCert, clientKey)
		if err != nil {
			slog.Error("failed to load client cert/key", "cert", clientCert, "key", clientKey, "err", err)
			return nil
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return &http.Client{
		Timeout: 0,
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
		},
	}
}

// ---------- kube-layer wiring ----------

// isCaptureEnabled reports whether spec.capture.enabled is set on the GameServer.
func isCaptureEnabled(gs *gameplanev1alpha1.GameServer) bool {
	return gs.Spec.Capture != nil && gs.Spec.Capture.Enabled
}

// hasActiveCapture reports whether this GameServer already has a
// Pending/Running capture. It checks the fast path (status.capture.active
// Capture, maintained by the operator) but falls back to scanning the
// server's own NetworkCaptures — the status field is eventually consistent
// (populated by the operator's reconciler), so relying on it alone would
// let a second capture-start race in before that field catches up.
func (h *captureHandler) hasActiveCapture(ctx context.Context, k *kube.Client, gs *gameplanev1alpha1.GameServer, ns, name string) (bool, error) {
	if gs.Status.Capture != nil && gs.Status.Capture.ActiveCapture != nil && *gs.Status.Capture.ActiveCapture != "" {
		return true, nil
	}
	captures, err := k.ListNetworkCaptures(ctx, ns, name)
	if err != nil {
		return false, err
	}
	for _, c := range captures {
		if c.Status.Phase == gameplanev1alpha1.CapturePhasePending || c.Status.Phase == gameplanev1alpha1.CapturePhaseRunning {
			return true, nil
		}
	}
	return false, nil
}

// resolveCapture fetches a NetworkCapture by ID and verifies it belongs to
// the named server, returning errCaptureNotFound (wrapped, via errors.Is)
// for either "no such capture" or "belongs to a different server" — the
// contract requires both to 404 identically.
func (h *captureHandler) resolveCapture(ctx context.Context, k *kube.Client, ns, name, captureID string) (*gameplanev1alpha1.NetworkCapture, error) {
	nc, err := k.GetNetworkCapture(ctx, ns, captureID)
	if err != nil {
		return nil, err
	}
	if nc == nil || nc.Spec.ServerRef.Name != name {
		return nil, errCaptureNotFound
	}
	return nc, nil
}

// expiresAt computes the display-only expiresAt field: completedAt plus
// the capture's own TTL (falling back to the cluster default retention
// when the capture didn't specify one). Empty until the capture has a
// completion time.
func (h *captureHandler) expiresAt(nc gameplanev1alpha1.NetworkCapture) string {
	if nc.Status.CompletionTime == nil {
		return ""
	}
	ttl := h.cfg.DefaultRetentionSeconds
	if nc.Spec.TTLSecondsAfterFinished != nil {
		ttl = int64(*nc.Spec.TTLSecondsAfterFinished)
	}
	return nc.Status.CompletionTime.Time.Add(time.Duration(ttl) * time.Second).UTC().Format(time.RFC3339)
}

// generateCaptureID mints a "cap-<8 hex chars>" identifier, matching the
// shape used throughout the contract's examples. crypto/rand failing means
// the OS CSPRNG is broken, which every other ID generator in this codebase
// (auth/sessions.go, db/shares.go) also treats as unrecoverable.
func generateCaptureID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	return "cap-" + hex.EncodeToString(b)
}

// maxFilterLength mirrors NetworkCaptureSpec.Filter's own CRD validation
// (operator/api/v1alpha1/networkcapture_types.go: MaxLength=1024) so an
// over-length filter 400s at the API tier instead of failing CRD admission
// after being echoed back to the client.
const maxFilterLength = 1024

// filterAllowedChars is the character set a pcap-filter(7) expression can
// legitimately use: identifiers, numbers, whitespace, and the small
// punctuation set the grammar's operators are built from.
const filterAllowedChars = " ._-:/()[]<>=!&|"

// validateFilter performs the API-tier syntactic check FR-003 requires
// ("invalid filter expressions MUST be rejected before the capture
// starts"). It intentionally does not attempt a full pcap-filter(7)
// grammar/BPF compile: the only pure-Go compiler for that
// (packetcap/go-pcap/filter) has no tagged release (research.md's "Open
// Risks" #1 — only a pseudo-version resolves from the module proxy) and is
// not part of this module's dependency graph. Pulling in an unverifiable,
// untagged third-party dependency here — without the ability to build and
// confirm the change compiles — would repeat the exact fabricated/unverified
// API risk this rework exists to eliminate. Instead this rejects the
// shapes that can never be a valid pcap-filter expression: empty-after-
// validation is skipped (empty means "no filter", the default applies),
// over-length, and characters pcap-filter syntax never uses.
func validateFilter(filter string) error {
	if filter == "" {
		return nil
	}
	if len(filter) > maxFilterLength {
		return fmt.Errorf("invalid filter: exceeds maximum length of %d characters", maxFilterLength)
	}
	for _, r := range filter {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case strings.ContainsRune(filterAllowedChars, r):
		default:
			return fmt.Errorf("invalid filter: unexpected character %q", r)
		}
	}
	return nil
}

func filterString(f *string) string {
	if f == nil {
		return ""
	}
	return *f
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func formatOptionalTime(t *metav1.Time) string {
	if t == nil {
		return ""
	}
	return formatTime(t.Time)
}

func quantityValue(q *resource.Quantity) int64 {
	if q == nil {
		return 0
	}
	return q.Value()
}

func maxDurationSecondsOf(d *metav1.Duration) int {
	if d == nil {
		return 0
	}
	return int(d.Duration.Seconds())
}

// ---------- SSRF-safe agent proxy helpers (mirrors ws/dialer.go; kept
// unexported and duplicated here since dialer.go's equivalents are
// unexported and this package cannot import them) ----------

// isDNS1123Label reports whether name is a valid DNS-1123 label (valid
// Kubernetes resource name component). Namespace and pod names flow into a
// constructed proxy URL, so both are validated against this rule before use
// to prevent SSRF injection. A label must consist of lower-case alphanumerics
// and hyphens, start and end with an alphanumeric, and be at most 63
// characters long.
func isDNS1123Label(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}
	for i, r := range name {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
		if i == 0 || i == len(name)-1 {
			if r == '-' {
				return false
			}
		}
	}
	return true
}

// proxyHeaderAllowlist is the set of request headers forwarded from the
// dashboard-facing API to the in-cluster agent. Anything not in this
// set is dropped — most importantly Cookie, Authorization, and the CSRF
// header, which are user-session material the agent doesn't need.
var proxyHeaderAllowlist = map[string]bool{
	"Accept":            true,
	"Accept-Encoding":   true,
	"Range":             true,
	"If-None-Match":     true,
	"If-Modified-Since": true,
}

// responseHeaderDenylist strips hop-by-hop and auth-adjacent response
// headers from the agent before passing the body through to the browser.
var responseHeaderDenylist = map[string]bool{
	"Set-Cookie":         true,
	"Www-Authenticate":   true,
	"Proxy-Authenticate": true,
	"Connection":         true,
	"Keep-Alive":         true,
	"Transfer-Encoding":  true,
	"Upgrade":            true,
}

func copyProxyHeaders(dst, src http.Header) {
	for k, v := range src {
		if !proxyHeaderAllowlist[http.CanonicalHeaderKey(k)] {
			continue
		}
		dst[http.CanonicalHeaderKey(k)] = v
	}
}

func copyResponseHeaders(dst, src http.Header) {
	for k, v := range src {
		if responseHeaderDenylist[http.CanonicalHeaderKey(k)] {
			continue
		}
		dst[http.CanonicalHeaderKey(k)] = v
	}
}
