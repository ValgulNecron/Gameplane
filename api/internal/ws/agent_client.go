package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AgentClient makes one-shot JSON GETs against agent sidecars over the same
// mTLS material the proxy uses. Handlers use it when they need agent data
// server-side (e.g. the mod update check reads the installed-mod manifest)
// instead of proxying a browser request through.
type AgentClient struct {
	http *http.Client
	// hostFn resolves the agent address; a field so tests can point it at
	// an httptest server.
	hostFn func(name, namespace string) string
}

// NewAgentClient builds a client from the agent mTLS flags. It fails when
// the material is missing/invalid — callers treat that like the proxy does
// (degrade to 503, don't crash startup).
func NewAgentClient(caBundle, clientCert, clientKey string) (*AgentClient, error) {
	tlsCfg, err := agentTLSConfig(caBundle, clientCert, clientKey)
	if err != nil {
		return nil, err
	}
	return &AgentClient{
		http: &http.Client{
			Timeout:   15 * time.Second,
			Transport: &http.Transport{TLSClientConfig: tlsCfg},
		},
		hostFn: agentHostFor,
	}, nil
}

// agentRespCap bounds one-shot agent responses. These are small JSON
// payloads (mod listings), nothing like the file proxy's traffic.
const agentRespCap = 8 << 20 // 8 MiB

// GetJSON fetches an agent endpoint and decodes its JSON body into out.
func (c *AgentClient) GetJSON(ctx context.Context, name, namespace, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://"+c.hostFn(name, namespace)+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("agent GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("agent GET %s: status %d", path, resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, agentRespCap)).Decode(out); err != nil {
		return fmt.Errorf("agent GET %s: decode: %w", path, err)
	}
	return nil
}

// agentHostFor is the in-cluster DNS + port of a server's agent sidecar —
// the operator maintains the <gs>-agent ClusterIP Service; the agent
// listens on :8090.
func agentHostFor(name, namespace string) string {
	return fmt.Sprintf("%s-agent.%s.svc.cluster.local:8090", name, namespace)
}
