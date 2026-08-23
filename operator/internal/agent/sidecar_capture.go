package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// CaptureClient speaks to the capture sidecar's :9091 HTTPS endpoints.
// It follows the same mTLS pattern as the agent Client.
type CaptureClient struct {
	http *http.Client
	// Disabled is true when no mTLS material was configured.
	Disabled bool
}

// NewCaptureClient builds an mTLS-configured CaptureClient from an existing agent Client.
// Since both use the same mTLS material and the same Service DNS, we reuse the http.Client.
func NewCaptureClient(agent *Client) *CaptureClient {
	if agent == nil || agent.Disabled {
		return &CaptureClient{Disabled: true}
	}
	return &CaptureClient{
		http:     agent.http,
		Disabled: agent.Disabled,
	}
}

// sidecarURL builds the in-cluster URL for a given GameServer's capture sidecar,
// via the dedicated `<gs>-agent` ClusterIP Service. The sidecar listens on port 9091.
func sidecarURL(namespace, server, path string) string {
	return fmt.Sprintf("https://%s-agent.%s.svc.cluster.local:9091%s", server, namespace, path)
}

// startCaptureRequest is the body sent to POST /captures/{id}/start.
type startCaptureRequest struct {
	Filter              *string `json:"filter,omitempty"`
	MaxDurationSeconds  int64   `json:"maxDurationSeconds"`
	MaxSizeBytes        int64   `json:"maxSizeBytes"`
}

// startCaptureResponse is returned by POST /captures/{id}/start.
type startCaptureResponse struct {
	Status         string `json:"status"`
	CaptureID      string `json:"captureId"`
	StartedAt      string `json:"startedAt"`
	BytesWritten   int64  `json:"bytesWritten"`
	PacketsWritten int64  `json:"packetsWritten"`
}

// StartCapture calls POST /captures/{id}/start on the sidecar.
func (c *CaptureClient) StartCapture(
	ctx context.Context,
	namespace, server, captureID string,
	filter *string,
	maxDurationSeconds, maxSizeBytes int64,
) error {
	if c.Disabled {
		return nil
	}

	body := startCaptureRequest{
		Filter:             filter,
		MaxDurationSeconds: maxDurationSeconds,
		MaxSizeBytes:       maxSizeBytes,
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("capture sidecar: marshal start request: %w", err)
	}

	url := sidecarURL(namespace, server, fmt.Sprintf("/captures/%s/start", captureID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("capture sidecar: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("capture sidecar: start capture: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("capture sidecar: start capture: status %d: %s", resp.StatusCode, string(respBody))
	}

	var respParsed startCaptureResponse
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &respParsed); err != nil {
			return fmt.Errorf("capture sidecar: start capture: decode response: %w", err)
		}
	}

	return nil
}

// stopCaptureRequest is the body sent to POST /captures/{id}/stop.
type stopCaptureRequest struct {
	Reason string `json:"reason"`
}

// StopCapture calls POST /captures/{id}/stop on the sidecar.
func (c *CaptureClient) StopCapture(
	ctx context.Context,
	namespace, server, captureID string,
) error {
	if c.Disabled {
		return nil
	}

	body := stopCaptureRequest{Reason: "user_requested"}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("capture sidecar: marshal stop request: %w", err)
	}

	url := sidecarURL(namespace, server, fmt.Sprintf("/captures/%s/stop", captureID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("capture sidecar: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("capture sidecar: stop capture: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("capture sidecar: stop capture: status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// getCaptureStatusResponse is returned by GET /captures/{id}/status.
type getCaptureStatusResponse struct {
	CaptureID      string `json:"captureId"`
	Status         string `json:"status"`
	StartedAt      string `json:"startedAt"`
	CompletedAt    string `json:"completedAt"`
	StoppingReason string `json:"stoppingReason"`
	BytesWritten   int64  `json:"bytesWritten"`
	PacketsWritten int64  `json:"packetsWritten"`
}

// GetCaptureStatus calls GET /captures/{id}/status on the sidecar.
// Returns the status phase, packet count, byte count, message, and error.
func (c *CaptureClient) GetCaptureStatus(
	ctx context.Context,
	namespace, server, captureID string,
) (string, int64, int64, string, error) {
	if c.Disabled {
		return "unknown", 0, 0, "", fmt.Errorf("capture sidecar client disabled")
	}

	url := sidecarURL(namespace, server, fmt.Sprintf("/captures/%s/status", captureID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("capture sidecar: create request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("capture sidecar: get status: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))

	if resp.StatusCode != http.StatusOK {
		return "", 0, 0, "", fmt.Errorf("capture sidecar: get status: status %d", resp.StatusCode)
	}

	var respParsed getCaptureStatusResponse
	if err := json.Unmarshal(respBody, &respParsed); err != nil {
		return "", 0, 0, "", fmt.Errorf("capture sidecar: get status: decode response: %w", err)
	}

	message := fmt.Sprintf("capture %s, packets=%d, bytes=%d", respParsed.Status, respParsed.PacketsWritten, respParsed.BytesWritten)

	return respParsed.Status, respParsed.PacketsWritten, respParsed.BytesWritten, message, nil
}
