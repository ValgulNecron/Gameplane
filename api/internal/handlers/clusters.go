// Clusters wires the multi-cluster registration surface:
//
//   - GET    /clusters              — list registered remote clusters
//   - POST   /clusters              — register a new remote cluster
//   - DELETE /clusters/{name}       — unregister a cluster

package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

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

type clustersHandler struct {
	reg       *kube.Registry
	k         *kube.Client
	namespace string
}

// clusterRegistryView is the public projection of a remote cluster. Never includes kubeconfig data.
type clusterRegistryView struct {
	Name          string `json:"name"`
	DisplayName   string `json:"displayName"`
	Phase         string `json:"phase"`
	Message       string `json:"message,omitempty"`
	ServerVersion string `json:"serverVersion,omitempty"`
	LastCheckTime string `json:"lastCheckTime,omitempty"`
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
	out := clustersListResp{Items: make([]clusterRegistryView, 0)}

	for _, id := range h.reg.IDs() {
		if id == h.reg.DefaultID() {
			// Synthesize the local cluster without reading from the API.
			out.Items = append(out.Items, clusterRegistryView{
				Name:  id,
				Phase: "Healthy",
			})
			continue
		}

		// Read remote cluster status from the Cluster CRD.
		u, err := h.k.Dynamic.Resource(kube.GVRCluster).Get(req.Context(), id, metav1.GetOptions{})
		if err != nil {
			// Log but continue: a missing CRD doesn't block the list.
			continue
		}

		displayName, _, _ := unstructured.NestedString(u.Object, "spec", "displayName")
		phase, _, _ := unstructured.NestedString(u.Object, "status", "phase")
		message, _, _ := unstructured.NestedString(u.Object, "status", "message")
		serverVersion, _, _ := unstructured.NestedString(u.Object, "status", "serverVersion")
		lastCheckTime, _, _ := unstructured.NestedString(u.Object, "status", "lastCheckTime")

		out.Items = append(out.Items, clusterRegistryView{
			Name:          id,
			DisplayName:   displayName,
			Phase:         phase,
			Message:       message,
			ServerVersion: serverVersion,
			LastCheckTime: lastCheckTime,
		})
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
	if !nameRE.MatchString(in.Name) {
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

	// Create the kubeconfig Secret in the control-plane namespace.
	secretName := "cluster-" + in.Name + "-kubeconfig"
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: h.namespace,
			Labels: map[string]string{
				kube.ClusterKubeconfigLabel: "true",
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

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, clusterRegistryView{
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

	// Clean up the kubeconfig Secret.
	if secretName != "" {
		_ = h.k.Typed.CoreV1().Secrets(h.namespace).Delete(req.Context(), secretName, metav1.DeleteOptions{})
	}

	w.WriteHeader(http.StatusNoContent)
}
