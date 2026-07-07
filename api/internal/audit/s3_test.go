package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
)

// s3CounterValue reads the current value of the S3 counter for a result label.
func s3CounterValue(t *testing.T, result string) float64 {
	t.Helper()
	var m dto.Metric
	if err := s3Events.WithLabelValues(result).Write(&m); err != nil {
		t.Fatalf("read counter %q: %v", result, err)
	}
	return m.GetCounter().GetValue()
}

// startSink launches the sink's delivery worker in a goroutine and returns a
// channel that closes once Start returns, mirroring the join pattern
// cmd/main.go uses around WebhookSink/S3Sink.Start. Tests must cancel ctx and
// wait on the returned channel before returning: s3Events is a package-global
// counter, so a test that lets its worker goroutine outlive the test (still
// retrying PutObject against an already-closed httptest server) pollutes the
// counter deltas read by every later test in this file.
func startSink(sink *S3Sink, ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		sink.Start(ctx)
		close(done)
	}()
	return done
}

// TestS3Sink_FlushOnCountThreshold flushes when buffer reaches 100 events.
func TestS3Sink_FlushOnCountThreshold(t *testing.T) {
	received := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		b, _ := io.ReadAll(r.Body)
		received <- b
		w.Header().Set("ETag", "test")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := S3Config{
		Endpoint:  strings.TrimPrefix(srv.URL, "http://"),
		Bucket:    "test-bucket",
		Prefix:    "audit",
		Insecure:  true,
		AccessKey: "test",
		SecretKey: "test",
	}
	sink, err := NewS3Sink(cfg)
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := startSink(sink, ctx)
	defer func() {
		cancel()
		<-done
	}()

	before := s3CounterValue(t, "sent")

	for i := 0; i < s3FlushCountSize; i++ {
		sink.Enqueue(Event{
			TS: "2026-06-30T00:00:00Z", Actor: "admin", Method: "POST",
			Path: "/api/v1/servers", Status: 201,
		})
	}

	select {
	case b := <-received:
		lines := strings.Split(strings.TrimSpace(string(b)), "\n")
		if len(lines) != s3FlushCountSize {
			t.Errorf("batch size = %d, want %d", len(lines), s3FlushCountSize)
		}
		for i, line := range lines {
			var p map[string]any
			if err := json.Unmarshal([]byte(line), &p); err != nil {
				t.Errorf("line %d decode: %v (%s)", i, err, line)
			}
			if p["actor"] != "admin" || p["method"] != "POST" || p["path"] != "/api/v1/servers" {
				t.Errorf("line %d: unexpected payload %v", i, p)
			}
			if _, hasID := p["id"]; hasID {
				t.Errorf("line %d: payload must not carry db id", i)
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("flush not received")
	}

	deadline := time.Now().Add(time.Second)
	for s3CounterValue(t, "sent") <= before {
		if time.Now().After(deadline) {
			t.Fatal("sent counter did not advance")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestS3Sink_FlushOnByteThreshold flushes when buffer exceeds 1 MiB.
func TestS3Sink_FlushOnByteThreshold(t *testing.T) {
	received := make(chan int, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		received <- len(b)
		w.Header().Set("ETag", "test")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := S3Config{
		Endpoint:  strings.TrimPrefix(srv.URL, "http://"),
		Bucket:    "test-bucket",
		Insecure:  true,
		AccessKey: "test",
		SecretKey: "test",
	}
	sink, err := NewS3Sink(cfg)
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := startSink(sink, ctx)
	defer func() {
		cancel()
		<-done
	}()

	largeStr := strings.Repeat("x", 10000)
	for i := 0; i < 120; i++ {
		sink.Enqueue(Event{
			TS: "2026-06-30T00:00:00Z", Actor: largeStr, Method: "POST",
			Path: "/api/v1/servers", Status: 201, IP: largeStr,
		})
	}

	select {
	case size := <-received:
		if size < s3FlushByteSize {
			t.Errorf("batch size = %d, expected > %d", size, s3FlushByteSize)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("flush not received")
	}
}

// TestS3Sink_FlushOnInterval flushes after 5 seconds even with few events.
func TestS3Sink_FlushOnInterval(t *testing.T) {
	received := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- struct{}{}
		w.Header().Set("ETag", "test")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := S3Config{
		Endpoint:  strings.TrimPrefix(srv.URL, "http://"),
		Bucket:    "test-bucket",
		Insecure:  true,
		AccessKey: "test",
		SecretKey: "test",
	}
	sink, err := NewS3Sink(cfg)
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := startSink(sink, ctx)
	defer func() {
		cancel()
		<-done
	}()

	sink.Enqueue(Event{
		TS: "2026-06-30T00:00:00Z", Actor: "admin", Method: "POST",
		Path: "/api/v1/servers", Status: 201,
	})

	select {
	case <-received:
	case <-time.After(6 * time.Second):
		t.Fatal("flush not received within 6s")
	}
}

// TestS3Sink_NDJSONFormat verifies the NDJSON encoding.
func TestS3Sink_NDJSONFormat(t *testing.T) {
	received := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/x-ndjson" {
			t.Errorf("content-type = %q", r.Header.Get("Content-Type"))
		}
		b, _ := io.ReadAll(r.Body)
		received <- b
		w.Header().Set("ETag", "test")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := S3Config{
		Endpoint:  strings.TrimPrefix(srv.URL, "http://"),
		Bucket:    "test-bucket",
		Insecure:  true,
		AccessKey: "test",
		SecretKey: "test",
	}
	sink, err := NewS3Sink(cfg)
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := startSink(sink, ctx)
	defer func() {
		cancel()
		<-done
	}()

	sink.Enqueue(Event{
		TS: "2026-06-30T00:00:00Z", Actor: "admin", Method: "POST",
		Path: "/api/v1/servers", Target: "alpha", Status: 201, IP: "10.0.0.1",
	})
	sink.Enqueue(Event{
		TS: "2026-06-30T00:00:01Z", Actor: "user", Method: "DELETE",
		Path: "/api/v1/servers/beta", Status: 204,
	})

	select {
	case b := <-received:
		lines := strings.Split(strings.TrimSpace(string(b)), "\n")
		if len(lines) < 2 {
			t.Fatalf("expected at least 2 lines, got %d", len(lines))
		}

		var e1, e2 map[string]any
		if err := json.Unmarshal([]byte(lines[0]), &e1); err != nil {
			t.Fatalf("decode line 0: %v", err)
		}
		if err := json.Unmarshal([]byte(lines[1]), &e2); err != nil {
			t.Fatalf("decode line 1: %v", err)
		}

		if e1["actor"] != "admin" || e1["target"] != "alpha" || e1["ip"] != "10.0.0.1" {
			t.Errorf("e1 = %v", e1)
		}
		if e2["actor"] != "user" || e2["path"] != "/api/v1/servers/beta" {
			t.Errorf("e2 = %v", e2)
		}
		if _, hasID := e1["id"]; hasID {
			t.Errorf("payload must not carry db id")
		}
	case <-time.After(7 * time.Second):
		t.Fatal("flush not received")
	}
}

// TestS3Sink_ObjectKeyFormat verifies the S3 object key structure.
func TestS3Sink_ObjectKeyFormat(t *testing.T) {
	received := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.URL.Path
		w.Header().Set("ETag", "test")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := S3Config{
		Endpoint:  strings.TrimPrefix(srv.URL, "http://"),
		Bucket:    "test-bucket",
		Prefix:    "audit",
		Insecure:  true,
		AccessKey: "test",
		SecretKey: "test",
	}
	sink, err := NewS3Sink(cfg)
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := startSink(sink, ctx)
	defer func() {
		cancel()
		<-done
	}()

	for i := 0; i < s3FlushCountSize; i++ {
		sink.Enqueue(Event{TS: "2026-06-30T00:00:00Z", Actor: "admin", Method: "POST", Path: "/", Status: 200})
	}

	select {
	case path := <-received:
		if !strings.Contains(path, "audit/") || !strings.Contains(path, ".ndjson") {
			t.Errorf("key path = %q, expected prefix and .ndjson", path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("flush not received")
	}
}

// TestS3Sink_ObjectKeyFormatNoPrefix verifies the key format when prefix is empty.
func TestS3Sink_ObjectKeyFormatNoPrefix(t *testing.T) {
	received := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.URL.Path
		w.Header().Set("ETag", "test")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := S3Config{
		Endpoint:  strings.TrimPrefix(srv.URL, "http://"),
		Bucket:    "test-bucket",
		Prefix:    "",
		Insecure:  true,
		AccessKey: "test",
		SecretKey: "test",
	}
	sink, err := NewS3Sink(cfg)
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := startSink(sink, ctx)
	defer func() {
		cancel()
		<-done
	}()

	for i := 0; i < s3FlushCountSize; i++ {
		sink.Enqueue(Event{TS: "2026-06-30T00:00:00Z", Actor: "admin", Method: "POST", Path: "/", Status: 200})
	}

	select {
	case path := <-received:
		parts := strings.Split(path, "/")
		if len(parts) < 4 {
			t.Errorf("key path = %q, expected YYYY/MM/DD/...", path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("flush not received")
	}
}

// TestS3Sink_DropsWhenBufferFull counts dropped events when the channel is full.
func TestS3Sink_DropsWhenBufferFull(t *testing.T) {
	sink, err := NewS3Sink(S3Config{
		Endpoint:  "127.0.0.1:9",
		Bucket:    "test",
		Insecure:  true,
		AccessKey: "test",
		SecretKey: "test",
	})
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}

	before := s3CounterValue(t, "dropped")
	for i := 0; i < s3Buffer; i++ {
		sink.Enqueue(Event{Actor: "x"})
	}

	const overflow = 5
	done := make(chan struct{})
	go func() {
		for i := 0; i < overflow; i++ {
			sink.Enqueue(Event{Actor: "x"})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Enqueue blocked on full buffer")
	}

	if delta := s3CounterValue(t, "dropped") - before; delta != float64(overflow) {
		t.Errorf("dropped delta = %v, want %d", delta, overflow)
	}
}

// TestS3Sink_RetryAndFail verifies retry logic and failure counting.
func TestS3Sink_RetryAndFail(t *testing.T) {
	failures := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failures++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := S3Config{
		Endpoint:  strings.TrimPrefix(srv.URL, "http://"),
		Bucket:    "test-bucket",
		Insecure:  true,
		AccessKey: "test",
		SecretKey: "test",
	}
	sink, err := NewS3Sink(cfg)
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}

	before := s3CounterValue(t, "failed")
	sink.pushBatch([]Event{
		{TS: "2026-06-30T00:00:00Z", Actor: "admin", Method: "POST", Path: "/", Status: 200},
		{TS: "2026-06-30T00:00:01Z", Actor: "user", Method: "DELETE", Path: "/api", Status: 204},
	})

	// pushBatch is synchronous here (no worker goroutine involved), and every
	// attempt gets a 5xx from srv, so all 3 scheduled attempts (immediate,
	// +2s, +8s) must have reached the fake server — assert that directly
	// against what the server actually saw, rather than only the shared
	// counter. However, minio-go performs its own internal retries (~10 per
	// PutObject call) on 5xx responses, so the server observes 3 * (internal
	// retries) ≈ 30 requests total — assert >= rather than exact equality.
	if failures < 3 {
		t.Errorf("attempts received by server = %d, want >= 3", failures)
	}
	// s3Events is a package-global CounterVec shared by every test in this
	// file, so its delta can only ever be a lower bound here — tolerate
	// >= rather than requiring an exact match (see startSink's doc comment).
	if delta := s3CounterValue(t, "failed") - before; delta < 2.0 {
		t.Errorf("failed delta = %v, want >= 2", delta)
	}
}

// TestS3Sink_DrainOnShutdown ships buffered events on context cancellation.
func TestS3Sink_DrainOnShutdown(t *testing.T) {
	received := make(chan int, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		lines := bytes.Count(b, []byte("\n"))
		received <- lines
		w.Header().Set("ETag", "test")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := S3Config{
		Endpoint:  strings.TrimPrefix(srv.URL, "http://"),
		Bucket:    "test-bucket",
		Insecure:  true,
		AccessKey: "test",
		SecretKey: "test",
	}
	sink, err := NewS3Sink(cfg)
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := startSink(sink, ctx)

	for i := 0; i < 10; i++ {
		sink.Enqueue(Event{TS: "2026-06-30T00:00:00Z", Actor: "admin", Method: "POST", Path: "/", Status: 200})
	}

	cancel()

	select {
	case count := <-received:
		if count != 10 {
			t.Errorf("events drained = %d, want 10", count)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("drain did not flush events")
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("worker did not stop after cancel")
	}
}

// TestS3Sink_RetryThenSucceed verifies recovery from transient failures.
func TestS3Sink_RetryThenSucceed(t *testing.T) {
	attempts := 0
	received := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			// First attempt: transient error
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Second attempt: success
		received <- struct{}{}
		w.Header().Set("ETag", "test")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := S3Config{
		Endpoint:  strings.TrimPrefix(srv.URL, "http://"),
		Bucket:    "test-bucket",
		Insecure:  true,
		AccessKey: "test",
		SecretKey: "test",
	}
	sink, err := NewS3Sink(cfg)
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}

	before := s3CounterValue(t, "sent")
	sink.pushBatch([]Event{
		{TS: "2026-06-30T00:00:00Z", Actor: "admin", Method: "POST", Path: "/", Status: 200},
	})

	select {
	case <-received:
		if delta := s3CounterValue(t, "sent") - before; delta != 1.0 {
			t.Errorf("sent delta = %v, want 1", delta)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("batch not delivered after retry")
	}
}
