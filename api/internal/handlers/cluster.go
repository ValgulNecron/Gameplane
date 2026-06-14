package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kestrel-gg/kestrel/api/internal/db"
	"github.com/kestrel-gg/kestrel/api/internal/httperr"
	"github.com/kestrel-gg/kestrel/api/internal/kube"
)

// MountCluster exposes read-only cluster observability the dashboard's
// Cluster page and header rely on. Everything here reads live Kubernetes
// state via the API's in-cluster client — no caching, no side effects —
// keeping the operator authoritative. All routes are GETs (viewer+ under
// the RBAC rules in api/internal/rbac).
func MountCluster(r chi.Router, k *kube.Client, store *db.Store) {
	h := &clusterHandler{k: k, store: store}
	r.Get("/cluster", h.view)
	r.Get("/cluster/info", h.info)
	r.Get("/cluster/stats", h.stats)
}

type clusterHandler struct {
	k     *kube.Client
	store *db.Store
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

// resourcePair mirrors the dashboard's {used?, capacity?} shape. `used`
// is omitted when unknown (the API has no metrics-server dependency), so
// the UI renders capacity and a 0% utilization bar rather than a wrong
// number.
type resourcePair struct {
	Used     *int64 `json:"used,omitempty"`
	Capacity int64  `json:"capacity"`
}

type clusterView struct {
	Nodes   []clusterNode `json:"nodes"`
	Version string        `json:"version,omitempty"`
	Name    string        `json:"name,omitempty"`
	Ready   int           `json:"ready"`
	Total   int           `json:"total"`
}

type clusterInfo struct {
	ClusterName string `json:"clusterName,omitempty"`
	Version     string `json:"version,omitempty"`
}

type clusterStats struct {
	Nodes             int   `json:"nodes"`
	TotalStorageBytes int64 `json:"totalStorageBytes"`
	UsedStorageBytes  int64 `json:"usedStorageBytes"`
}

func (h *clusterHandler) view(w http.ResponseWriter, req *http.Request) {
	nodes, err := h.k.Typed.CoreV1().Nodes().List(req.Context(), metav1.ListOptions{})
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	out := clusterView{
		Nodes:   make([]clusterNode, 0, len(nodes.Items)),
		Version: h.serverVersion(),
		Name:    h.clusterName(req.Context()),
		Total:   len(nodes.Items),
	}
	for i := range nodes.Items {
		n := mapNode(&nodes.Items[i])
		if n.Status == "Ready" {
			out.Ready++
		}
		out.Nodes = append(out.Nodes, n)
	}
	writeJSON(w, out)
}

func (h *clusterHandler) info(w http.ResponseWriter, req *http.Request) {
	writeJSON(w, clusterInfo{
		ClusterName: h.clusterName(req.Context()),
		Version:     h.serverVersion(),
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
	var total, used int64
	for i := range pvs.Items {
		cap := pvs.Items[i].Spec.Capacity[corev1.ResourceStorage]
		total += cap.Value()
		// "Used" here means provisioned-and-bound storage — the closest
		// signal available without a metrics pipeline.
		if pvs.Items[i].Status.Phase == corev1.VolumeBound {
			used += cap.Value()
		}
	}
	writeJSON(w, clusterStats{
		Nodes:             len(nodes.Items),
		TotalStorageBytes: total,
		UsedStorageBytes:  used,
	})
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
