package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/ValgulNecron/gameplane/api/internal/kube"
)

func readyNode(name string, ready bool, cpu, mem string) *corev1.Node {
	cond := corev1.ConditionFalse
	if ready {
		cond = corev1.ConditionTrue
	}
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"node-role.kubernetes.io/control-plane": ""},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: cond}},
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(cpu),
				corev1.ResourceMemory: resource.MustParse(mem),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
		},
	}
}

func boundPV(name, size string, bound bool) *corev1.PersistentVolume {
	phase := corev1.VolumeAvailable
	if bound {
		phase = corev1.VolumeBound
	}
	return &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)},
		},
		Status: corev1.PersistentVolumeStatus{Phase: phase},
	}
}

func TestCluster_View(t *testing.T) {
	cs := fake.NewSimpleClientset(
		readyNode("cp-0", true, "4", "8Gi"),
		readyNode("worker-0", false, "8", "16Gi"),
	)
	cs.Discovery().(*fakediscovery.FakeDiscovery).FakedServerVersion = &version.Info{GitVersion: "v1.31.2"}
	k := &kube.Client{Typed: cs}

	r := chi.NewRouter()
	MountCluster(r, k, nil, "v9.9.9-test")

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/cluster", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var view clusterView
	if err := json.Unmarshal(rr.Body.Bytes(), &view); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if view.Total != 2 || view.Ready != 1 {
		t.Fatalf("ready/total = %d/%d, want 1/2", view.Ready, view.Total)
	}
	if view.Version != "v1.31.2" {
		t.Fatalf("version = %q, want v1.31.2", view.Version)
	}
	if len(view.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2", len(view.Nodes))
	}
	var cp *clusterNode
	for i := range view.Nodes {
		if view.Nodes[i].Name == "cp-0" {
			cp = &view.Nodes[i]
		}
	}
	if cp == nil || cp.Status != "Ready" || cp.CPU == nil || cp.CPU.Capacity != 4 {
		t.Fatalf("cp-0 mapped wrong: %+v", cp)
	}
	if len(cp.Roles) != 1 || cp.Roles[0] != "control-plane" {
		t.Fatalf("cp-0 roles = %v, want [control-plane]", cp.Roles)
	}
}

func TestCluster_Stats(t *testing.T) {
	cs := fake.NewSimpleClientset(
		readyNode("cp-0", true, "4", "8Gi"),
		boundPV("pv-bound", "10Gi", true),
		boundPV("pv-free", "5Gi", false),
	)
	k := &kube.Client{Typed: cs}

	r := chi.NewRouter()
	MountCluster(r, k, nil, "v9.9.9-test")

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/cluster/stats", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var stats clusterStats
	if err := json.Unmarshal(rr.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if stats.Nodes != 1 {
		t.Fatalf("nodes = %d, want 1", stats.Nodes)
	}
	const gi = 1024 * 1024 * 1024
	if stats.TotalStorageBytes != 15*gi {
		t.Fatalf("total = %d, want %d", stats.TotalStorageBytes, int64(15*gi))
	}
	if stats.UsedStorageBytes != 10*gi {
		t.Fatalf("used = %d, want %d", stats.UsedStorageBytes, int64(10*gi))
	}
}

func TestCluster_Info_NilStore(t *testing.T) {
	cs := fake.NewSimpleClientset()
	cs.Discovery().(*fakediscovery.FakeDiscovery).FakedServerVersion = &version.Info{GitVersion: "v1.30.0"}
	k := &kube.Client{Typed: cs}

	r := chi.NewRouter()
	MountCluster(r, k, nil, "v9.9.9-test")

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/cluster/info", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	var info clusterInfo
	if err := json.Unmarshal(rr.Body.Bytes(), &info); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if info.Version != "v1.30.0" {
		t.Fatalf("version = %q", info.Version)
	}
	if info.GameplaneVersion != "v9.9.9-test" {
		t.Fatalf("gameplaneVersion = %q, want v9.9.9-test", info.GameplaneVersion)
	}
	if info.ClusterName != "" {
		t.Fatalf("clusterName = %q, want empty with nil store", info.ClusterName)
	}
}
