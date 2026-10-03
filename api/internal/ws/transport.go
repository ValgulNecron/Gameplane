package ws

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/coder/websocket"
)

// agentTarget identifies a server within the cluster served by a transport.
// The caller selects and authorizes the cluster before using the transport.
type agentTarget struct {
	name      string
	namespace string
}

func (t agentTarget) validate() error {
	if !isDNS1123Label(t.namespace) || !isDNS1123Label(t.name) {
		return errors.New("invalid namespace or pod name")
	}
	return nil
}

// agentRequest contains only the agent operation, never a caller-supplied
// destination URL. Proxy handlers remain responsible for body limits.
type agentRequest struct {
	target   agentTarget
	method   string
	path     string
	rawQuery string
	header   http.Header
	body     io.Reader
}

// agentTransport separates agent operations from their network route. A
// transport is scoped to one cluster; a missing remote transport must never
// fall back to the local one. The initial implementation preserves the
// existing direct connection inside the home cluster.
type agentTransport interface {
	Do(context.Context, agentRequest) (*http.Response, error)
	Dial(context.Context, agentTarget, string) (*websocket.Conn, *http.Response, error)
}

type directAgentTransport struct {
	http   *http.Client
	hostFn func(name, namespace string) string
}

func newDirectAgentTransport(tlsCfg *tls.Config, timeout time.Duration) *directAgentTransport {
	return &directAgentTransport{
		http: &http.Client{
			Timeout:   timeout,
			Transport: &http.Transport{TLSClientConfig: tlsCfg},
			// Agent operations must stay on the selected server. A redirect
			// must not send credentials or a mutation to another destination.
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		hostFn: agentHostFor,
	}
}

func (t *directAgentTransport) endpoint(target agentTarget, scheme, path, rawQuery string) (string, error) {
	if err := target.validate(); err != nil {
		return "", err
	}
	// Paths originate from registered operations, not from user input. Keeping
	// query and path separate prevents a query from replacing the authority.
	u := &url.URL{
		Scheme:   scheme,
		Host:     t.hostFn(target.name, target.namespace),
		Path:     path,
		RawQuery: rawQuery,
	}
	return u.String(), nil
}

func (t *directAgentTransport) Do(ctx context.Context, operation agentRequest) (*http.Response, error) {
	endpoint, err := t.endpoint(operation.target, "https", operation.path, operation.rawQuery)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, operation.method, endpoint, operation.body)
	if err != nil {
		return nil, err
	}
	// Keep browser session material out of agents even for internal callers.
	copyProxyHeaders(req.Header, operation.header)
	return t.http.Do(req)
}

func (t *directAgentTransport) Dial(ctx context.Context, target agentTarget, path string) (*websocket.Conn, *http.Response, error) {
	endpoint, err := t.endpoint(target, "wss", path, "")
	if err != nil {
		return nil, nil, err
	}
	return websocket.Dial(ctx, endpoint, &websocket.DialOptions{HTTPClient: t.http})
}
