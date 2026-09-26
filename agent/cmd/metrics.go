package main

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// newMetricsServer builds the listener that serves Prometheus metrics. It is
// separate from the mTLS control listener (--addr), which requires a client
// cert signed by --tls-client-ca (or a bearer token) for every route it
// serves, so a Prometheus scraper never needs that trust to reach metrics —
// only /metrics is routed here, in the clear, and every other path answers
// 404. The chart's agent PodMonitor targets this listener's containerPort
// (named "metrics" by the operator; see buildAgentContainer).
func newMetricsServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
}
