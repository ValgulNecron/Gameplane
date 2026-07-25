package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ValgulNecron/gameplane/test/e2e/internal/probe"
)

// statusResponse mirrors the HTTP status.json response from the
// lloesche/valheim-server image, per
// https://github.com/lloesche/valheim-server-docker#status-http-server
type statusResponse struct {
	Server struct {
		Players      int    `json:"players"`
		MaxPlayers   int    `json:"max_players"`
		WorldSaveAge int    `json:"world_save_age"`
		UptimeSecs   int    `json:"uptime_secs"`
		Version      string `json:"version"`
		NetworkInfo  struct {
			ConnectedPlayers int `json:"connected_players"`
		} `json:"network_info"`
	} `json:"server"`
}

func main() {
	flags := probe.ParseFlags()
	probe.Main(flags, func(ctx context.Context) (probe.Depth, error) {
		return probeValheim(ctx, flags.Addr)
	})
}

// probeValheim fetches the documented status.json HTTP endpoint from a
// Valheim server and returns QUERY depth on success.
//
// Valheim's game protocol (UDP 2456+) is not publicly documented, so a
// full join (JOINED or PARTIAL depth) is not possible in CI. However,
// the lloesche/valheim-server image exposes a documented HTTP status
// endpoint at /status.json (port 80 by default). A successful fetch and
// parse proves the server is alive and responding to the health protocol
// that Kubernetes uses for readiness probes.
func probeValheim(ctx context.Context, addr string) (probe.Depth, error) {
	// Retry fetching status.json at 15 seconds per attempt.
	var status *statusResponse
	if err := probe.Retry(ctx, "status.json", 15*time.Second, func(actx context.Context) error {
		var err error
		status, err = fetchStatus(actx, addr)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return "", err
	}

	// Log server metadata from status response as evidence.
	if status != nil && status.Server.Version != "" {
		fmt.Printf("valheim status: players=%d/%d uptime=%ds version=%s\n",
			status.Server.Players, status.Server.MaxPlayers,
			status.Server.UptimeSecs, status.Server.Version)
	}

	return probe.Query, nil
}

// fetchStatus fetches and parses status.json from the Valheim HTTP status
// endpoint (port 80 by default). The addr parameter is host:port of the
// game server's status port.
func fetchStatus(ctx context.Context, addr string) (*statusResponse, error) {
	// Construct the HTTP GET request to /status.json.
	// addr is "host:port" where port is STATUS_HTTP_PORT (default 80).
	url := fmt.Sprintf("http://%s/status.json", addr)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Use a 10-second timeout for the HTTP request itself (independent of
	// the overall deadline managed by probe.Retry).
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req = req.WithContext(reqCtx)

	// Send the request.
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch status.json: %w", err)
	}
	defer resp.Body.Close()

	// Check HTTP status code.
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status.json returned %d", resp.StatusCode)
	}

	// Read and parse the JSON response.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var status statusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("parse status.json: %w", err)
	}

	return &status, nil
}
