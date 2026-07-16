package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8srest "k8s.io/client-go/rest"

	"github.com/ValgulNecron/gameplane/api/internal/db"
	"github.com/ValgulNecron/gameplane/api/internal/httperr"
	"github.com/ValgulNecron/gameplane/api/internal/kube"
)

// MountCluster exposes read-only cluster observability the dashboard's
// Cluster page and header rely on. Everything here reads live Kubernetes
// state via the API's in-cluster client — no caching, no side effects —
// keeping the operator authoritative. All routes are GETs (viewer+ under
// the RBAC rules in api/internal/rbac).
func MountCluster(r chi.Router, k *kube.Client, store *db.Store, gameplaneVersion string, clusterOps bool, updateChannel string) {
	h := &clusterHandler{k: k, store: store, gameplaneVersion: gameplaneVersion, clusterOps: clusterOps, updateChannel: updateChannel}
	r.Get("/cluster", h.view)
	r.Get("/cluster/info", h.info)
	r.Get("/cluster/stats", h.stats)
}

type clusterHandler struct {
	k              *kube.Client
	store          *db.Store
	gameplaneVersion string
	clusterOps       bool
	updateChannel    string
}

type clusterNode struct {
	Name      string        `json:"name"`
	Roles     []string      `json:"roles,omitempty"`
	Status    string        `json:"status"`
	StartedAt string        `json:"startedAt,omitempty"`
	Pods      *resourcePair `json:"pods,omitempty"`
	CPU       *resourcePair `json:"cpu,omitempty"`
	Memory    *resourcePair `json:"memory,omitempty"`
}

// resourcePair mirrors the dashboard's {used?, capacity?} shape. `used` is
// populated from metrics-server (the metrics.k8s.io aggregated API) when
// it's installed and reachable; it's omitted — not zero — when unknown, so
// the UI can tell "measured idle" apart from "no metrics pipeline" instead
// of rendering a false 0% bar either way. Capacity and used always share a
// unit per resource: CPU is whole/fractional cores (matching how capacity
// was already rendered — "N cores" — so used can carry metrics-server's
// sub-core precision without a wider unit change rippling through the
// dashboard), memory is bytes, pods is a plain count.
type resourcePair struct {
	Used     *float64 `json:"used,omitempty"`
	Capacity int64    `json:"capacity"`
}

type clusterView struct {
	Nodes   []clusterNode `json:"nodes"`
	Version string        `json:"version,omitempty"`
	Name    string        `json:"name,omitempty"`
	Ready   int           `json:"ready"`
	Total   int           `json:"total"`
}

type clusterInfo struct {
	ClusterName    string `json:"clusterName,omitempty"`
	Version        string `json:"version,omitempty"`        // Kubernetes server version
	GameplaneVersion string `json:"gameplaneVersion,omitempty"` // Gameplane control-plane build
	// ClusterOps mirrors the --cluster-ops flag so the dashboard can
	// grey out node-join / kubeconfig actions instead of letting every
	// click run into the handlers' 501. Never omitted: the client must
	// distinguish "off" from "server too old to report it".
	ClusterOps bool `json:"clusterOps"`
	// UpdateChannel is the chart's informational updates.channel label.
	// Nothing consumes it server-side — upgrades happen via Helm — it
	// exists so the dashboard can show which channel this install tracks.
	UpdateChannel string `json:"updateChannel,omitempty"`
}

type clusterStats struct {
	Nodes int `json:"nodes"`
	// TotalStorageBytes is the physical disk the cluster's nodes report
	// (ephemeral-storage capacity) — the denominator the web client
	// compares provisioned storage against.
	TotalStorageBytes int64 `json:"totalStorageBytes"`
	// UsedStorageBytes is NOT disk usage — Kubernetes exposes no such
	// metric without a metrics/monitoring pipeline. It is storage
	// *provisioned*: the capacity of Bound PersistentVolumes, i.e. what has
	// been handed out to workloads regardless of how full those volumes
	// actually are. With networked storage (Ceph, EBS, …) PVs don't come
	// off node disk, so this can legitimately exceed TotalStorageBytes —
	// an honest overcommit signal, not a bug. The JSON field name stays
	// "usedStorageBytes" for API compatibility; the web client is
	// responsible for labeling it "provisioned" and presenting the
	// overcommit case explicitly rather than as a >100% bar.
	UsedStorageBytes int64 `json:"usedStorageBytes"`
}

func (h *clusterHandler) view(w http.ResponseWriter, req *http.Request) {
	nodes, err := h.k.Typed.CoreV1().Nodes().List(req.Context(), metav1.ListOptions{})
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	usage := h.fetchNodeUsage(req.Context())
	out := clusterView{
		Nodes:   make([]clusterNode, 0, len(nodes.Items)),
		Version: h.serverVersion(),
		Name:    h.clusterName(req.Context()),
		Total:   len(nodes.Items),
	}
	for i := range nodes.Items {
		n := mapNode(&nodes.Items[i])
		if u, ok := usage[n.Name]; ok {
			if n.CPU != nil {
				cpuUsed := u.cpuCores
				n.CPU.Used = &cpuUsed
			}
			if n.Memory != nil {
				memUsed := u.memoryBytes
				n.Memory.Used = &memUsed
			}
		}
		if n.Status == "Ready" {
			out.Ready++
		}
		out.Nodes = append(out.Nodes, n)
	}
	writeJSON(w, out)
}

// nodeUsage is one node's live CPU/memory consumption as reported by
// metrics-server, already converted to the same units resourcePair.Capacity
// uses (cores, bytes) so callers can assign it straight to `Used`.
type nodeUsage struct {
	cpuCores    float64
	memoryBytes float64
}

// metricsNodeList is the subset of the metrics.k8s.io/v1beta1 NodeMetrics
// list response this handler needs. Defined locally instead of importing
// k8s.io/metrics for one read-only GET.
type metricsNodeList struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Usage struct {
			CPU    string `json:"cpu"`
			Memory string `json:"memory"`
		} `json:"usage"`
	} `json:"items"`
}

// fetchNodeUsage reads live per-node CPU/memory usage from the
// metrics.k8s.io aggregated API (metrics-server) through the existing typed
// client's raw REST path — this repo avoids adding k8s.io/metrics as a
// dependency for a single GET.
//
// Returns nil, never an error: a cluster without metrics-server installed
// is a normal, supported deployment (the operator doesn't require one), so
// any failure to reach or parse the endpoint — 404 because the API group
// isn't registered, a network error, a malformed body — must leave the
// caller with "no usage data" rather than fail the whole /cluster request.
// Logged at debug, not warn/error, since this is an expected steady state
// on many installs.
func (h *clusterHandler) fetchNodeUsage(ctx context.Context) map[string]nodeUsage {
	rc := h.k.Typed.CoreV1().RESTClient()
	// Guards against a typed-nil interface: fake Kubernetes clientsets
	// (used throughout this package's tests) return a nil *rest.RESTClient
	// wrapped in a non-nil rest.Interface, which would otherwise panic on
	// the first field access inside Get(). A real cluster's client never
	// returns nil here.
	if rc == nil {
		return nil
	}
	if typed, ok := rc.(*k8srest.RESTClient); ok && typed == nil {
		return nil
	}

	raw, err := rc.Get().AbsPath("/apis/metrics.k8s.io/v1beta1/nodes").DoRaw(ctx)
	if err != nil {
		slog.Debug("metrics-server unavailable, cluster CPU/memory usage omitted", "err", err)
		return nil
	}

	var parsed metricsNodeList
	if err := json.Unmarshal(raw, &parsed); err != nil {
		slog.Debug("metrics-server response unparsable, cluster CPU/memory usage omitted", "err", err)
		return nil
	}

	usage := make(map[string]nodeUsage, len(parsed.Items))
	for _, item := range parsed.Items {
		cpu, err := resource.ParseQuantity(item.Usage.CPU)
		if err != nil {
			continue
		}
		mem, err := resource.ParseQuantity(item.Usage.Memory)
		if err != nil {
			continue
		}
		// MilliValue() first (millicores), then down to cores: matches the
		// unit resourcePair.Capacity already uses for CPU while keeping
		// metrics-server's sub-core precision instead of rounding a small
		// usage figure up to a whole core.
		usage[item.Metadata.Name] = nodeUsage{
			cpuCores:    float64(cpu.MilliValue()) / 1000,
			memoryBytes: float64(mem.Value()),
		}
	}
	return usage
}

func (h *clusterHandler) info(w http.ResponseWriter, req *http.Request) {
	writeJSON(w, clusterInfo{
		ClusterName:    h.clusterName(req.Context()),
		Version:        h.serverVersion(),
		GameplaneVersion: h.gameplaneVersion,
		ClusterOps:       h.clusterOps,
		UpdateChannel:    h.updateChannel,
	})
}

func (h *clusterHandler) stats(w http.ResponseWriter, req *http.Request) {
	nodes, err := h.k.Typed.CoreV1().Nodes().List(req.Context(), metav1.ListOptions{})
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	pvs, err := h.k.Typed.CoreV1().PersistentVolumes().List(req.Context(), metav1.ListOptions{})
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	writeJSON(w, clusterStats{
		Nodes:             len(nodes.Items),
		TotalStorageBytes: nodeStorageCapacity(nodes.Items),
		UsedStorageBytes:  boundVolumeBytes(pvs.Items),
	})
}

// nodeStorageCapacity sums the ephemeral-storage capacity each node reports:
// the physical disk behind the cluster.
//
// Measuring "total" as the sum of *all* PersistentVolume capacity — as this
// once did — makes the storage meter degenerate. Every PV that exists is
// almost always Bound, so used == total and the meter reads 100% on every
// install. Nodes give an independent denominator.
//
// Caveat: with networked storage (Ceph, EBS) the volumes do not come off the
// node disks, so provisioned storage can exceed this figure. That reads as
// >100% — an honest signal of overcommit rather than a silent 100%.
func nodeStorageCapacity(nodes []corev1.Node) int64 {
	var total int64
	for i := range nodes {
		q := nodes[i].Status.Capacity[corev1.ResourceEphemeralStorage]
		total += q.Value()
	}
	return total
}

// boundVolumeBytes sums the capacity of Bound PersistentVolumes. An unbound
// volume is spare capacity, not consumption.
func boundVolumeBytes(pvs []corev1.PersistentVolume) int64 {
	var used int64
	for i := range pvs {
		if pvs[i].Status.Phase != corev1.VolumeBound {
			continue
		}
		q := pvs[i].Spec.Capacity[corev1.ResourceStorage]
		used += q.Value()
	}
	return used
}

// serverVersion returns the cluster's Kubernetes version (e.g.
// "v1.31.0"), or "" if discovery fails — the UI tolerates an empty
// version.
func (h *clusterHandler) serverVersion() string {
	v, err := h.k.Typed.Discovery().ServerVersion()
	if err != nil || v == nil {
		return ""
	}
	return v.GitVersion
}

// clusterName returns the operator-configured instance name from the
// admin "general" config section, or "" when unset. This is the
// user-facing cluster label (Kubernetes has no native cluster name).
func (h *clusterHandler) clusterName(ctx context.Context) string {
	if h.store == nil {
		return ""
	}
	raw, ok, err := h.store.ConfigValue(ctx, "general")
	if err != nil || !ok {
		return ""
	}
	var g struct {
		InstanceName string `json:"instanceName"`
	}
	if err := json.Unmarshal([]byte(raw), &g); err != nil {
		return ""
	}
	return g.InstanceName
}

func mapNode(n *corev1.Node) clusterNode {
	out := clusterNode{
		Name:      n.Name,
		Roles:     nodeRoles(n),
		Status:    "NotReady",
		StartedAt: n.CreationTimestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	for _, c := range n.Status.Conditions {
		if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
			out.Status = "Ready"
			break
		}
	}
	if cpu, ok := n.Status.Capacity[corev1.ResourceCPU]; ok {
		out.CPU = &resourcePair{Capacity: cpu.Value()}
	}
	if mem, ok := n.Status.Capacity[corev1.ResourceMemory]; ok {
		out.Memory = &resourcePair{Capacity: mem.Value()}
	}
	if pods, ok := n.Status.Capacity[corev1.ResourcePods]; ok {
		out.Pods = &resourcePair{Capacity: pods.Value()}
	}
	return out
}

// nodeRoles extracts roles from the standard node-role.kubernetes.io/<r>
// labels.
func nodeRoles(n *corev1.Node) []string {
	var roles []string
	const prefix = "node-role.kubernetes.io/"
	for k := range n.Labels {
		if strings.HasPrefix(k, prefix) {
			if r := strings.TrimPrefix(k, prefix); r != "" {
				roles = append(roles, r)
			}
		}
	}
	sort.Strings(roles)
	return roles
}
