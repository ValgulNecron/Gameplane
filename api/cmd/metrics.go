package main

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// newMetricsServer builds the listener that serves Prometheus metrics. It is
// separate from the public API listener, which the ingress and the web front
// end route to, so metrics are reachable only on the in-cluster scrape port
// (the chart's api.metricsPort, which the ServiceMonitor targets). Only
// /metrics is routed; every other path answers 404.
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
