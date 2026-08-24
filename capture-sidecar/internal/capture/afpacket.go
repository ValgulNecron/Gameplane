// Package capture implements live packet capture for the network capture
// sidecar: reading frames off an AF_PACKET socket, compiling and applying a
// BPF filter expression over them, and writing the matched packets out as a
// PCAPNG file.
package capture

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gopacket/gopacket/afpacket"
	"golang.org/x/net/bpf"
)

// pollTimeout bounds how long a single AF_PACKET poll blocks before
// ReadPacket rechecks ctx.Done()/closed and retries.
//
// gopacket/afpacket has no native read cancellation: TPacket.ReadPacketData
// only returns once a packet arrives, the poll(2) call errors, or the poll
// times out (returning the exported afpacket.ErrTimeout). Passing a short,
// finite afpacket.OptPollTimeout is what makes ctx cancellation and Close
// responsive, without spinning up a goroutine per read that can outlive its
// caller if the underlying blocking syscall never returns on its own.
const pollTimeout = 250 * time.Millisecond

// PacketSource is an interface for reading packets. This allows tests to
// inject mock packet sources without requiring root access or a real network
// interface. Real captures use the AF_PACKET implementation; tests can provide
// synthetic packets.
type PacketSource interface {
	// ReadPacket returns the next packet and its receive timestamp.
	// It blocks until a packet is available or the context is canceled.
	// Returns nil packet and non-nil error on context cancellation or I/O error.
	ReadPacket(ctx context.Context) (*RawPacket, error)
	// Close closes the packet source and releases any resources.
	Close() error
}

// RawPacket represents a single captured network packet.
type RawPacket struct {
	// Data is the packet bytes as captured, already truncated to the
	// source's snaplen.
	Data []byte

	// Timestamp is when the packet was received.
	Timestamp time.Time

	// OriginalLength is the length of the frame on the wire, before any
	// snaplen truncation. It is what a PCAPNG reader reports as the packet's
	// real size, so it must not be derived from len(Data). Zero means the
	// source did not report one.
	OriginalLength int
}

// AFPacketSource implements PacketSource using Linux AF_PACKET sockets
// with an mmap'd TPacket buffer for efficient live capture.
type AFPacketSource struct {
	handle  *afpacket.TPacket
	snaplen uint32

	mu     sync.Mutex
	closed bool
}

// NewAFPacketSource creates a new AF_PACKET packet source listening on
// the specified interface with the given snaplen (maximum captured packet size).
// The BPF filter (if non-empty) is compiled and loaded into the kernel.
func NewAFPacketSource(iface string, snaplen uint32, bpfFilter string) (*AFPacketSource, error) {
	if snaplen == 0 {
		snaplen = DefaultSnaplen
	}

	// Open an AF_PACKET TPacket socket bound to the specified interface.
	// The interface is selected via afpacket.OptInterface at construction
	// time; TPacket has no SetInterface/SetSnaplen setters (verified against
	// github.com/gopacket/gopacket@v1.6.1/afpacket/{afpacket,options}.go:
	// NewTPacket at afpacket.go:261, OptInterface at options.go:80,
	// OptPollTimeout at options.go:100).
	// OptPollTimeout bounds each internal poll(2) call so ReadPacketData
	// returns periodically (afpacket.ErrTimeout) instead of blocking forever,
	// which is what ReadPacket below relies on for ctx cancellation.
	handle, err := afpacket.NewTPacket(
		afpacket.OptInterface(iface),
		afpacket.OptPollTimeout(pollTimeout),
	)
	if err != nil {
		return nil, fmt.Errorf("open AF_PACKET socket on %s: %w", iface, err)
	}

	// If a BPF filter is provided, compile it and load the resulting program
	// into the kernel via SetBPF. CompileFilter produces a []bpf.Instruction
	// (github.com/packetcap/go-pcap/filter's portable representation);
	// bpf.Assemble converts that into the []bpf.RawInstruction SetBPF wants
	// (golang.org/x/net/bpf.Assemble, verified against golang.org/x/net@v0.57.0/bpf/asm.go).
	if bpfFilter != "" {
		f, err := CompileFilter(bpfFilter)
		if err != nil {
			handle.Close()
			return nil, fmt.Errorf("compile BPF filter: %w", err)
		}
		raw, err := bpf.Assemble(f.Instructions())
		if err != nil {
			handle.Close()
			return nil, fmt.Errorf("assemble BPF filter: %w", err)
		}
		if err := handle.SetBPF(raw); err != nil {
			handle.Close()
			return nil, fmt.Errorf("set BPF filter: %w", err)
		}
	}

	return &AFPacketSource{
		handle:  handle,
		snaplen: snaplen,
	}, nil
}

// ReadPacket reads the next available packet from the AF_PACKET socket.
// It blocks until a packet arrives or the context is canceled.
//
// TPacket.ReadPacketData has no cancellation of its own, so this loops on
// the bounded-timeout poll configured in NewAFPacketSource: every pollTimeout
// interval with no packet, ReadPacketData returns afpacket.ErrTimeout, and
// the loop rechecks ctx.Done()/closed before polling again. This keeps
// cancellation responsive without ever leaving a goroutine blocked on a
// syscall past the point the caller stopped waiting for it.
func (a *AFPacketSource) ReadPacket(ctx context.Context) (*RawPacket, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context canceled: %w", ctx.Err())
		default:
		}

		a.mu.Lock()
		if a.closed {
			a.mu.Unlock()
			return nil, fmt.Errorf("packet source is closed")
		}
		handle := a.handle
		a.mu.Unlock()

		data, ci, err := handle.ReadPacketData()
		if err != nil {
			if errors.Is(err, afpacket.ErrTimeout) {
				// No packet within pollTimeout; loop back and recheck
				// ctx/closed rather than treating this as a read failure.
				continue
			}
			return nil, fmt.Errorf("read packet: %w", err)
		}

		if a.snaplen > 0 && int64(len(data)) > int64(a.snaplen) {
			data = data[:a.snaplen]
		}
		// ci.Length is the frame's length on the wire (afpacket fills it from
		// the ring-buffer header's tp_len, not from the captured slice), so it
		// survives both the kernel's snaplen and the truncation above.
		return &RawPacket{
			Data:           data,
			Timestamp:      ci.Timestamp,
			OriginalLength: ci.Length,
		}, nil
	}
}

// Close closes the AF_PACKET socket and releases the mmap'd buffer.
func (a *AFPacketSource) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return nil
	}
	a.closed = true

	// (*afpacket.TPacket).Close returns no value (verified against
	// github.com/gopacket/gopacket@v1.6.1/afpacket/afpacket.go:243); it just
	// munmaps the ring buffer and closes the fd.
	a.handle.Close()
	return nil
}

// Capturer manages packet capture from a PacketSource.
// It reads packets from the source and delivers them to the Writer for recording.
type Capturer struct {
	source PacketSource
	mu     sync.Mutex
	closed bool
}

// NewCapturer creates a new Capturer using the provided PacketSource.
// This design allows tests to inject mock sources without requiring root or a real interface.
func NewCapturer(source PacketSource) *Capturer {
	return &Capturer{
		source: source,
	}
}

// Start validates that the capturer may begin: it rejects a capturer that is
// already closed and honors an already-canceled/expired ctx up front, so a
// caller that raced a start against its own shutdown gets a clear error
// instead of silently proceeding. The actual packet reads happen in
// ReadPackets, which takes its own ctx and is what makes cancellation of an
// in-progress capture responsive.
func (c *Capturer) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("capturer is closed")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("start capturer: %w", err)
	}
	return nil
}

// Stop closes the Capturer.
func (c *Capturer) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	if err := c.source.Close(); err != nil {
		return fmt.Errorf("close packet source: %w", err)
	}
	return nil
}

// ReadPackets reads packets from the source and delivers them to a callback.
// It runs until the context is canceled or an error occurs.
func (c *Capturer) ReadPackets(ctx context.Context, callback func(*RawPacket) error) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("capturer is closed")
	}
	c.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		pkt, err := c.source.ReadPacket(ctx)
		if err != nil {
			return fmt.Errorf("read packet: %w", err)
		}

		if err := callback(pkt); err != nil {
			return fmt.Errorf("packet callback failed: %w", err)
		}
	}
}
