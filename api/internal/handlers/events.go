package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kestrel-gg/kestrel/api/internal/kube"
)

// MountEvents exposes /events as a Server-Sent Events stream that
// mirrors Kubernetes watch events on all four Kestrel CRDs. The
// dashboard uses this to keep its cache fresh without polling.
func MountEvents(r chi.Router, k *kube.Client) {
	r.Get("/events", eventsHandler(k))
}

func eventsHandler(k *kube.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ctx := req.Context()
		for path, gvr := range kube.GVRs {
			path, gvr := path, gvr
			go func() {
				watcher, err := k.Dynamic.Resource(gvr).Namespace(metav1.NamespaceAll).
					Watch(ctx, metav1.ListOptions{})
				if err != nil {
					return
				}
				defer watcher.Stop()
				for ev := range watcher.ResultChan() {
					if ev.Type == "" {
						continue
					}
					u, ok := ev.Object.(*unstructured.Unstructured)
					if !ok {
						continue
					}
					payload := map[string]any{
						"kind":      path,
						"eventType": ev.Type,
						"object":    u.Object,
					}
					b, _ := json.Marshal(payload)
					fmt.Fprintf(w, "data: %s\n\n", b)
					flusher.Flush()
				}
			}()
		}

		<-ctx.Done()
	}
}
