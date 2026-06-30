// Command audit-syslog-bridge is a tiny, generic HTTP-JSON → syslog relay. It
// accepts an HTTP POST whose body is a single JSON document and re-emits that
// document as an RFC 5424 syslog message to a configured collector (TCP, TCP
// over TLS, or UDP).
//
// It exists so Gameplane's audit webhook sink (which POSTs each audit event as
// JSON) can reach a syslog/SIEM endpoint, but it is deliberately
// schema-agnostic: it forwards the received body verbatim as the syslog MSG, so
// it works for any JSON webhook source, not just audit events.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// RFC 5424 facility and severity name → numeric maps. PRI = facility*8 + severity.
var facilities = map[string]int{
	"kern": 0, "user": 1, "mail": 2, "daemon": 3, "auth": 4, "syslog": 5,
	"lpr": 6, "news": 7, "uucp": 8, "cron": 9, "authpriv": 10, "ftp": 11,
	"local0": 16, "local1": 17, "local2": 18, "local3": 19,
	"local4": 20, "local5": 21, "local6": 22, "local7": 23,
}

var severities = map[string]int{
	"emerg": 0, "alert": 1, "crit": 2, "err": 3,
	"warning": 4, "notice": 5, "info": 6, "debug": 7,
}

// Version is set at build time via -ldflags "-X main.Version=...".
var Version = "dev"

// maxBody bounds the inbound POST body. Audit events are small; this just stops
// a misdirected client from streaming something huge at us.
const maxBody = 64 << 10

type config struct {
	listen      string
	syslogAddr  string
	network     string // tcp | udp
	useTLS      bool
	appName     string
	facility    string
	severity    string
	hostname    string
	authHeader  string
	dialTimeout time.Duration
}

func loadConfig() config {
	return config{
		listen:      envOr("LISTEN_ADDR", ":8514"),
		syslogAddr:  envOr("SYSLOG_ADDR", ""),
		network:     envOr("SYSLOG_NETWORK", "tcp"),
		useTLS:      envOr("SYSLOG_TLS", "") == "true",
		appName:     envOr("APP_NAME", "gameplane-audit"),
		facility:    envOr("FACILITY", "local0"),
		severity:    envOr("SEVERITY", "info"),
		hostname:    envOr("SYSLOG_HOSTNAME", ""),
		authHeader:  envOr("AUTH_HEADER", ""),
		dialTimeout: 5 * time.Second,
	}
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

// server holds the resolved relay configuration and the syslog forwarder.
type server struct {
	pri        int
	appName    string
	hostname   string
	network    string
	authHeader string
	fwd        *forwarder
}

func newServer(cfg config) (*server, error) {
	if cfg.syslogAddr == "" {
		return nil, errors.New("SYSLOG_ADDR (-syslog-addr) is required")
	}
	if cfg.network != "tcp" && cfg.network != "udp" {
		return nil, fmt.Errorf("SYSLOG_NETWORK must be tcp or udp, got %q", cfg.network)
	}
	if cfg.useTLS && cfg.network != "tcp" {
		return nil, errors.New("SYSLOG_TLS requires SYSLOG_NETWORK=tcp")
	}
	fac, ok := facilities[cfg.facility]
	if !ok {
		return nil, fmt.Errorf("unknown FACILITY %q", cfg.facility)
	}
	sev, ok := severities[cfg.severity]
	if !ok {
		return nil, fmt.Errorf("unknown SEVERITY %q", cfg.severity)
	}
	host := cfg.hostname
	if host == "" {
		// Best-effort: a missing hostname is valid RFC 5424 ("-"), set below.
		host, _ = os.Hostname()
	}
	return &server{
		pri:        fac*8 + sev,
		appName:    cfg.appName,
		hostname:   host,
		network:    cfg.network,
		authHeader: cfg.authHeader,
		fwd:        newForwarder(cfg.network, cfg.syslogAddr, cfg.useTLS, cfg.dialTimeout),
	}, nil
}

func (s *server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/", s.handle)
	return mux
}

func (s *server) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.authHeader != "" && r.Header.Get("Authorization") != s.authHeader {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}
	line := buildSyslog(s.pri, time.Now(), s.hostname, s.appName, msg)
	if err := s.fwd.send(frameFor(s.network, line)); err != nil {
		slog.Error("forward to syslog failed", "err", err)
		http.Error(w, "syslog forward failed", http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// buildSyslog renders one RFC 5424 message:
//
//	<PRI>1 TIMESTAMP HOSTNAME APP-NAME PROCID MSGID STRUCTURED-DATA MSG
//
// PROCID, MSGID and STRUCTURED-DATA are nil ("-"); MSG carries the JSON body
// collapsed to a single line so it stays one syslog record.
func buildSyslog(pri int, ts time.Time, host, app, msg string) string {
	if host == "" {
		host = "-"
	}
	if app == "" {
		app = "-"
	}
	msg = strings.NewReplacer("\n", " ", "\r", " ").Replace(msg)
	return fmt.Sprintf("<%d>1 %s %s %s - - - %s",
		pri, ts.UTC().Format("2006-01-02T15:04:05.000Z07:00"), host, app, msg)
}

// frameFor wraps a syslog message for the wire. TCP uses RFC 6587
// octet-counting ("<len> <msg>") so a stream collector can split records
// unambiguously; UDP sends the bare message as one datagram.
func frameFor(network, msg string) []byte {
	if network == "tcp" {
		return []byte(strconv.Itoa(len(msg)) + " " + msg)
	}
	return []byte(msg)
}

// forwarder ships framed syslog messages over a lazily-dialed, reused
// connection. Writes are serialized; a failed write triggers one reconnect and
// retry before surfacing the error.
type forwarder struct {
	network     string
	addr        string
	useTLS      bool
	dialTimeout time.Duration

	mu   sync.Mutex
	conn net.Conn
}

func newForwarder(network, addr string, useTLS bool, dialTimeout time.Duration) *forwarder {
	return &forwarder{network: network, addr: addr, useTLS: useTLS, dialTimeout: dialTimeout}
}

func (f *forwarder) dial() error {
	var (
		c   net.Conn
		err error
	)
	if f.useTLS && f.network == "tcp" {
		d := &net.Dialer{Timeout: f.dialTimeout}
		c, err = tls.DialWithDialer(d, "tcp", f.addr, &tls.Config{MinVersion: tls.VersionTLS12})
	} else {
		c, err = net.DialTimeout(f.network, f.addr, f.dialTimeout)
	}
	if err != nil {
		return fmt.Errorf("dial %s/%s: %w", f.network, f.addr, err)
	}
	f.conn = c
	return nil
}

func (f *forwarder) send(frame []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.conn == nil {
		if err := f.dial(); err != nil {
			return err
		}
	}
	if _, err := f.conn.Write(frame); err != nil {
		// The collector may have dropped a long-idle connection; reconnect once.
		_ = f.conn.Close()
		f.conn = nil
		if err := f.dial(); err != nil {
			return err
		}
		if _, err := f.conn.Write(frame); err != nil {
			_ = f.conn.Close()
			f.conn = nil
			return fmt.Errorf("write syslog %s/%s: %w", f.network, f.addr, err)
		}
	}
	return nil
}

func run(cfg config) error {
	s, err := newServer(cfg)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              cfg.listen,
		Handler:           s.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	slog.Info("audit-syslog-bridge listening",
		"version", Version, "listen", cfg.listen, "syslog", cfg.syslogAddr,
		"network", cfg.network, "tls", cfg.useTLS, "app", cfg.appName)

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	}
}

func main() {
	if err := run(loadConfig()); err != nil {
		slog.Error("audit-syslog-bridge exited", "err", err)
		os.Exit(1)
	}
}
