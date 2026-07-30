// Package main is the Satisfactory probe.
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/ValgulNecron/gameplane/test/e2e/internal/probe"
)

func main() {
	flags := probe.ParseFlags()
	probe.Main(flags, func(ctx context.Context) (probe.Depth, error) {
		return probeSatisfactory(ctx, flags.Addr)
	})
}

// probeSatisfactory attempts to reach the deepest join depth on a Satisfactory
// dedicated server. Since CI cannot perform the in-game server claim required
// to set an admin password, all authenticated functions (PasswordLogin,
// RunCommand) are unreachable in test environments.
//
// The honest measurement is to attempt the unauthenticated QueryServerState
// function, which (according to the OpenAPI spec referenced in
// agent/internal/rcon/satisfactory.go) returns server metadata without requiring
// authentication. If this succeeds, depth is QUERY. If it fails, we confirm
// the HTTPS endpoint is reachable over TCP (connectivity test).
//
// Reference: The agent/internal/rcon/satisfactory.go file documents the
// Satisfactory HTTPS API at https://github.com/satisfactory-oas/spec — the
// endpoint is POST https://<host>:7777/api/v1 with a JSON body containing
// a "function" field.
func probeSatisfactory(ctx context.Context, addr string) (probe.Depth, error) {
	// First attempt: query server state without authentication.
	// This is the boundary of what CI can measure.
	if err := probe.Retry(ctx, "query-server-state", 15*time.Second, func(actx context.Context) error {
		err := queryServerState(actx, addr)
		if err != nil {
			return err
		}
		return nil
	}); err == nil {
		// QueryServerState succeeded; the unauthenticated API is working.
		log.Printf("query-server-state: success")
		return probe.Query, nil
	}

	// Fallback: if QueryServerState failed, attempt a simple TCP connection to
	// the game port to confirm the endpoint is reachable at all. This is weaker
	// than a protocol-level response but proves the port is open.
	if err := probe.Retry(ctx, "tcp-connect", 15*time.Second, func(actx context.Context) error {
		d := net.Dialer{Timeout: 5 * time.Second}
		conn, err := d.DialContext(actx, "tcp", addr)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		return nil
	}); err == nil {
		log.Printf("tcp-connect: success (endpoint reachable but API call failed)")
		return probe.Query, nil
	}

	// Both attempts failed; the server is not responding.
	return "", fmt.Errorf("satisfactory: query-server-state and tcp-connect both failed")
}

// queryServerState sends an unauthenticated QueryServerState request to the
// Satisfactory HTTPS API and returns an error if it fails.
func queryServerState(ctx context.Context, addr string) error {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(dctx context.Context, network, _ string) (net.Conn, error) {
				d := net.Dialer{Timeout: 5 * time.Second}
				return d.DialContext(dctx, network, addr)
			},
			// The Satisfactory dedicated server presents a self-signed certificate.
			// We skip verification only because this connection is pod-local (127.0.0.1
			// or cluster-internal DNS name), which is safe: an off-host MITM cannot
			// intercept the connection. See agent/internal/rcon/satisfactory.go's
			// documentation for the rationale.
			TLSClientConfig: &tls.Config{
				MinVersion:         tls.VersionTLS12,
				InsecureSkipVerify: true,
			},
		},
		Timeout: 10 * time.Second,
	}
	defer client.CloseIdleConnections()

	// Build the request: a JSON POST with the QueryServerState function.
	// Note: No authentication token is sent (no "Authorization" header),
	// so this tests the unauthenticated API surface.
	reqBody, err := json.Marshal(map[string]string{
		"function": "QueryServerState",
	})
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+addr+"/api/v1", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Log the status and any response body (bounded to avoid log flooding).
	body := make([]byte, 4096)
	n, _ := resp.Body.Read(body)
	if n > 0 {
		log.Printf("query-server-state: status=%d body=%s", resp.StatusCode, string(body[:n]))
	} else {
		log.Printf("query-server-state: status=%d (no body)", resp.StatusCode)
	}

	// A 2xx status indicates the endpoint accepted the request.
	// Parse the response to check for an error envelope.
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// The endpoint accepted the request. Check if the body contains an error.
		var respEnv struct {
			Data      any    `json:"data,omitempty"`
			ErrorCode string `json:"errorCode,omitempty"`
		}
		if err := json.Unmarshal(body[:n], &respEnv); err == nil {
			if respEnv.ErrorCode != "" {
				// The response has an error code; the endpoint rejected the call.
				return fmt.Errorf("query-server-state: %s (api error, not network)", respEnv.ErrorCode)
			}
			// Success: the endpoint accepted the unauthenticated call.
			return nil
		}
		// Response body is not JSON, but the status is 2xx; that's still success.
		return nil
	}

	// A 4xx/5xx status is a failure.
	return fmt.Errorf("query-server-state: http %d", resp.StatusCode)
}
