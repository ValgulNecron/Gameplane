package capture

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakePacketSource is an in-memory PacketSource used to exercise Capturer
// without requiring root, CAP_NET_RAW, or a real network interface.
type fakePacketSource struct {
	mu      sync.Mutex
	packets []*RawPacket
	idx     int
	closed  bool
	closeCh chan struct{}
}

func newFakePacketSource(packets []*RawPacket) *fakePacketSource {
	return &fakePacketSource{
		packets: packets,
		closeCh: make(chan struct{}),
	}
}

func (f *fakePacketSource) ReadPacket(ctx context.Context) (*RawPacket, error) {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return nil, errors.New("packet source is closed")
	}
	if f.idx < len(f.packets) {
		pkt := f.packets[f.idx]
		f.idx++
		f.mu.Unlock()
		return pkt, nil
	}
	f.mu.Unlock()

	// No more synthetic packets: block until the source is closed or the
	// caller's context is canceled, mirroring the real AFPacketSource's
	// blocking-until-cancel-or-data contract.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-f.closeCh:
		return nil, errors.New("packet source is closed")
	}
}

func (f *fakePacketSource) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil
	}
	f.closed = true
	close(f.closeCh)
	return nil
}

// TestCapturer_ReadPackets_DeliversAllPackets verifies that ReadPackets
// delivers every packet from the source to the callback, in order.
func TestCapturer_ReadPackets_DeliversAllPackets(t *testing.T) {
	want := []*RawPacket{
		{Data: []byte{0x01}, Timestamp: time.Now()},
		{Data: []byte{0x02}, Timestamp: time.Now()},
		{Data: []byte{0x03}, Timestamp: time.Now()},
	}
	source := newFakePacketSource(want)
	c := NewCapturer(source)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var got []*RawPacket
	var mu sync.Mutex
	done := make(chan error, 1)
	go func() {
		done <- c.ReadPackets(ctx, func(pkt *RawPacket) error {
			mu.Lock()
			got = append(got, pkt)
			shouldStop := len(got) == len(want)
			mu.Unlock()
			if shouldStop {
				cancel()
			}
			return nil
		})
	}()

	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("ReadPackets returned unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ReadPackets did not return after all packets were delivered")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(got) != len(want) {
		t.Fatalf("got %d packets; want %d", len(got), len(want))
	}
	for i := range want {
		if string(got[i].Data) != string(want[i].Data) {
			t.Errorf("packet %d: got %v; want %v", i, got[i].Data, want[i].Data)
		}
	}
}

// TestCapturer_Stop_UnblocksReadPackets verifies that Stop() closes the
// underlying source and causes an in-progress ReadPackets call to return,
// rather than blocking forever.
func TestCapturer_Stop_UnblocksReadPackets(t *testing.T) {
	source := newFakePacketSource(nil) // no packets; ReadPacket blocks immediately
	c := NewCapturer(source)

	ctx := context.Background()
	done := make(chan error, 1)
	go func() {
		done <- c.ReadPackets(ctx, func(*RawPacket) error { return nil })
	}()

	// Give the goroutine a moment to enter the blocking read.
	time.Sleep(50 * time.Millisecond)

	if err := c.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("ReadPackets returned nil error after Stop; want an error unblocking the read")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ReadPackets did not return after Stop; source close did not unblock it (goroutine leak)")
	}
}

// TestCapturer_Start_RejectsCanceledContext verifies that Start honors an
// already-canceled context rather than silently ignoring it.
func TestCapturer_Start_RejectsCanceledContext(t *testing.T) {
	c := NewCapturer(newFakePacketSource(nil))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := c.Start(ctx); err == nil {
		t.Fatal("Start with an already-canceled context succeeded; want error")
	}
}

// TestCapturer_Start_RejectsAfterStop verifies that Start fails once the
// capturer has been stopped.
func TestCapturer_Start_RejectsAfterStop(t *testing.T) {
	c := NewCapturer(newFakePacketSource(nil))
	if err := c.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if err := c.Start(context.Background()); err == nil {
		t.Fatal("Start after Stop succeeded; want error")
	}
}

// TestCapturer_Stop_Idempotent verifies that Stop can be called multiple
// times without error.
func TestCapturer_Stop_Idempotent(t *testing.T) {
	c := NewCapturer(newFakePacketSource(nil))
	for i := 0; i < 3; i++ {
		if err := c.Stop(); err != nil {
			t.Fatalf("Stop call %d failed: %v", i, err)
		}
	}
}
