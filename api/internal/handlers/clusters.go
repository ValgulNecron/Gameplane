// Clusters wires the multi-cluster registration surface:
//
//   - GET    /clusters              — list registered remote clusters
//   - POST   /clusters              — register a new remote cluster
//   - DELETE /clusters/{name}       — unregister a cluster

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/ValgulNecron/gameplane/api/internal/auth"
	"github.com/ValgulNecron/gameplane/api/internal/httperr"
	"github.com/ValgulNecron/gameplane/api/internal/kube"
)

// MountClusters wires /clusters onto the supplied router.
func MountClusters(r chi.Router, reg *kube.Registry, k *kube.Client, ns string) {
	h := clustersHandler{reg: reg, k: k, namespace: ns}
	r.Route("/clusters", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Delete("/{name}", h.delete)
	})
}

// clusterKubeconfigSecretName returns the Kubernetes Secret name that POST /clusters
// generates for a cluster's kubeconfig.
func clusterKubeconfigSecretName(cluster string) string {
	return "cluster-" + cluster + "-kubeconfig"
}

// deleteClusterKubeconfigSecret deletes a kubeconfig Secret only when it is the one
// that POST /clusters generates for this cluster (cluster-<name>-kubeconfig) and carries
// the kube.ClusterKubeconfigLabel label. Secrets created before managed-by labelling
// are cleaned up; any other Secret (different name or missing the kubeconfig label)
// is left in place and returns apierrors.NewNotFound.
func deleteClusterKubeconfigSecret(ctx context.Context, k *kube.Client, ns, cluster, secretName string) error {
	// Check if the secret name matches what POST /clusters generates.
	expectedName := clusterKubeconfigSecretName(cluster)
	if secretName != expectedName {
		return apierrors.NewNotFound(corev1.Resource("secrets"), secretName)
	}

	// Fetch the Secret to check its labels.
	secret, err := k.Typed.CoreV1().Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Only delete if it carries the kubeconfig label.
	if secret.Labels[kube.ClusterKubeconfigLabel] != "true" {
		return apierrors.NewNotFound(corev1.Resource("secrets"), secretName)
	}

	// Delete the Secret.
	return k.Typed.CoreV1().Secrets(ns).Delete(ctx, secretName, metav1.DeleteOptions{})
}

type clustersHandler struct {
	reg       *kube.Registry
	k         *kube.Client
	namespace string
}

// clusterRegistryView is the public projection of a remote cluster. Never includes kubeconfig data.
type clusterRegistryView struct {
	CanViewInventory bool   `json:"canViewInventory"`
	Name             string `json:"name"`
	DisplayName      string `json:"displayName"`
	Phase            string `json:"phase"`
	Message          string `json:"message,omitempty"`
	ServerVersion    string `json:"serverVersion,omitempty"`
	LastCheckTime    string `json:"lastCheckTime,omitempty"`
}

type clusterCreateReq struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Kubeconfig  string `json:"kubeconfig"`
}

type clustersListResp struct {
	Items []clusterRegistryView `json:"items"`
}

func (h clustersHandler) list(w http.ResponseWriter, req *http.Request) {
	u := auth.UserFromContext(req.Context())
	if u == nil {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	out := clustersListResp{Items: make([]clusterRegistryView, 0)}
	local := h.reg.DefaultID()
	if u.CanDiscoverCluster(local) {
		out.Items = append(out.Items, clusterRegistryView{Name: local, Phase: "Healthy",
			CanViewInventory: u.Can("cluster:read", true, local, "")})
	}
	// Persisted registrations remain discoverable when a kubeconfig cannot
	// load; the client registry alone would silently drop those clusters.
	registrations, err := h.k.Dynamic.Resource(kube.GVRCluster).List(req.Context(), metav1.ListOptions{})
	if err != nil {
		// Older single-cluster installations may not have the optional CRD.
		// Only that absence may degrade to local discovery; permission and
		// transport failures must remain visible instead of hiding remotes.
		if apierrors.IsNotFound(err) {
			writeJSON(w, out)
			return
		}
		httperr.Write(w, req, err)
		return
	}
	sort.Slice(registrations.Items, func(i, j int) bool {
		return registrations.Items[i].GetName() < registrations.Items[j].GetName()
	})
	for _, registration := range registrations.Items {
		id := registration.GetName()
		if id == local || registration.GetDeletionTimestamp() != nil || !u.CanDiscoverCluster(id) {
			continue
		}
		item := clusterRegistryView{Name: id, CanViewInventory: u.Can("cluster:read", true, id, "")}
		item.DisplayName, _, _ = unstructured.NestedString(registration.Object, "spec", "displayName")
		item.Phase, _, _ = unstructured.NestedString(registration.Object, "status", "phase")
		if item.CanViewInventory {
			item.Message, _, _ = unstructured.NestedString(registration.Object, "status", "message")
			item.ServerVersion, _, _ = unstructured.NestedString(registration.Object, "status", "serverVersion")
			item.LastCheckTime, _, _ = unstructured.NestedString(registration.Object, "status", "lastCheckTime")
		}
		if client, ok := h.reg.Get(id); !ok || client == nil || client.Typed == nil {
			item.Phase = "Unhealthy"
			item.Message = "Cluster connection is unavailable"
		}
		out.Items = append(out.Items, item)
	}

	writeJSON(w, out)
}

func (h clustersHandler) create(w http.ResponseWriter, req *http.Request) {
	var in clusterCreateReq
	if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
		httperr.Write(w, req, err)
		return
	}

	// Validate name.
	if !dnsLabelRE.MatchString(in.Name) {
		httperr.WriteCode(w, req, http.StatusBadRequest,
			errors.New("name must be a DNS label (lowercase, digits, hyphens)"))
		return
	}
	if in.Name == h.reg.DefaultID() || in.Name == "*" {
		httperr.WriteCode(w, req, http.StatusBadRequest,
			errors.New("cluster name conflicts with reserved names"))
		return
	}

	// Validate kubeconfig.
	if in.Kubeconfig == "" {
		httperr.WriteCode(w, req, http.StatusBadRequest,
			errors.New("kubeconfig is required"))
		return
	}
	_, err := kube.ConfigFromKubeconfig([]byte(in.Kubeconfig))
	if err != nil {
		httperr.WriteCode(w, req, http.StatusBadRequest, err)
		return
	}

	// Create the kubeconfig Secret in the control-plane namespace. The
	// managed-by label marks it as created by the API, which is what lets
	// DELETE /clusters/{name} remove it again.
	secretName := clusterKubeconfigSecretName(in.Name)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: h.namespace,
			Labels: map[string]string{
				kube.ClusterKubeconfigLabel: "true",
				ManagedByLabel:              managedByValue,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"kubeconfig": []byte(in.Kubeconfig),
		},
	}
	_, err = h.k.Typed.CoreV1().Secrets(h.namespace).Create(req.Context(), secret, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			httperr.WriteCode(w, req, http.StatusConflict, errors.New("cluster already exists"))
			return
		}
		httperr.Write(w, req, err)
		return
	}

	// Create the Cluster CR.
	clusterCR := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "gameplane.local/v1alpha1",
			"kind":       "Cluster",
			"metadata": map[string]any{
				"name": in.Name,
			},
			"spec": map[string]any{
				"displayName": in.DisplayName,
				"kubeconfigSecret": map[string]any{
					"name": secretName,
					"key":  "kubeconfig",
				},
			},
		},
	}
	_, err = h.k.Dynamic.Resource(kube.GVRCluster).Create(req.Context(), clusterCR, metav1.CreateOptions{})
	if err != nil {
		// Clean up the Secret on CR creation failure.
		_ = h.k.Typed.CoreV1().Secrets(h.namespace).Delete(req.Context(), secretName, metav1.DeleteOptions{})
		if apierrors.IsAlreadyExists(err) {
			httperr.WriteCode(w, req, http.StatusConflict, errors.New("cluster already exists"))
			return
		}
		httperr.Write(w, req, err)
		return
	}

	writeJSONCreated(w, clusterRegistryView{
		Name:        in.Name,
		DisplayName: in.DisplayName,
		Phase:       "", // Will be populated by the operator
	})
}

func (h clustersHandler) delete(w http.ResponseWriter, req *http.Request) {
	name := chi.URLParam(req, "name")

	// Reject deletion of the default cluster.
	if name == h.reg.DefaultID() {
		httperr.WriteCode(w, req, http.StatusBadRequest,
			errors.New("cannot delete the local cluster"))
		return
	}

	// Read the Cluster CR to find the Secret name.
	u, err := h.k.Dynamic.Resource(kube.GVRCluster).Get(req.Context(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			httperr.WriteCode(w, req, http.StatusNotFound, err)
			return
		}
		httperr.Write(w, req, err)
		return
	}

	// Extract the Secret name from the Cluster CR for cleanup.
	kcSpec, ok, _ := unstructured.NestedMap(u.Object, "spec", "kubeconfigSecret")
	secretName := ""
	if ok {
		if sn, ok := kcSpec["name"].(string); ok {
			secretName = sn
		}
	}

	// Delete the Cluster CR.
	if err := h.k.Dynamic.Resource(kube.GVRCluster).Delete(req.Context(), name, metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			httperr.Write(w, req, err)
			return
		}
	}

	// Drop the cluster's client now instead of waiting for the cluster
	// watch, so no request is dispatched through a removed registration.
	h.reg.Remove(name)

	// Clean up the kubeconfig Secret only if it is the one this cluster was created with
	// (cluster-<name>-kubeconfig) and carries the kubeconfig label. This includes Secrets
	// created before the managed-by label was added. Any other Secret — different name
	// or missing the kubeconfig label — is left in place.
	if secretName != "" {
		if err := deleteClusterKubeconfigSecret(req.Context(), h.k, h.namespace, name, secretName); err != nil && !apierrors.IsNotFound(err) {
			slog.Warn("cluster delete: kubeconfig secret cleanup failed", "cluster", name, "err", err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
