package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/ValgulNecron/gameplane/api/internal/httperr"
	"github.com/ValgulNecron/gameplane/api/internal/kube"
	"github.com/ValgulNecron/gameplane/api/internal/scope"
)

// MountEvents exposes /events as a Server-Sent Events stream mirroring
// Kubernetes watch events on the Gameplane CRDs, for clients that want
// cache-freshness without polling.
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
		// Scope the namespaced watches to the caller's permitted namespace
		// (viewers are pinned to the default). Previously this watched
		// metav1.NamespaceAll and streamed every namespace's objects —
		// including Secret refs / repo URLs — to any authenticated caller,
		// bypassing the scope check every other read path enforces.
		ns, err := scope.Resolve(req)
		if err != nil {
			httperr.Write(w, req, err)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ctx, cancel := context.WithCancel(req.Context())
		defer cancel()

		// Fan-in: per-CRD watch goroutines send framed events to one
		// writer. A single writer means no concurrent writes to the
		// ResponseWriter (the old code raced 4+ goroutines on it), and a
		// write error cancels ctx so every watcher stops (the old code
		// ignored write errors and leaked watchers on client disconnect).
		events := make(chan []byte, 32)
		var wg sync.WaitGroup
		for path, gvr := range kube.GVRs {
			path, gvr := path, gvr
			ri := k.Dynamic.Resource(gvr)
			var watcher watch.Interface
			if cluster(gvr) {
				watcher, err = ri.Watch(ctx, metav1.ListOptions{})
			} else {
				watcher, err = ri.Namespace(ns).Watch(ctx, metav1.ListOptions{})
			}
			if err != nil {
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer watcher.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case ev, ok := <-watcher.ResultChan():
						if !ok {
							return
						}
						if ev.Type == "" {
							continue
						}
						u, ok := ev.Object.(*unstructured.Unstructured)
						if !ok {
							continue
						}
						b, mErr := json.Marshal(map[string]any{
							"kind":      path,
							"eventType": ev.Type,
							"object":    u.Object,
						})
						if mErr != nil {
							continue
						}
						select {
						case events <- b:
						case <-ctx.Done():
							return
						}
					}
				}
			}()
		}
		// Closes the channel once all watchers have exited so the writer
		// loop below terminates cleanly.
		go func() { wg.Wait(); close(events) }()

		for b := range events {
			if _, werr := fmt.Fprintf(w, "data: %s\n\n", b); werr != nil {
				cancel() // client gone → stop the watchers
				break
			}
			flusher.Flush()
		}
	}
}
