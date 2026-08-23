package capture

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcapgo"
)

// PacketWriter is an interface for writing captured packets to a file.
// This allows tests to inject mock writers without requiring disk I/O.
// Real captures use the Writer implementation; tests can provide synthetic
// failures to exercise error handling paths.
type PacketWriter interface {
	// WritePacket writes a single packet to the capture file.
	// It returns an error wrapping ErrLimitReached once a hard limit is reached.
	WritePacket(pkt *RawPacket) error
	// Close finalizes the capture file and closes the underlying file handle.
	Close() error
	// BytesWritten returns the size of the capture file on disk.
	BytesWritten() int64
	// PacketsWritten returns the cumulative number of packets written.
	PacketsWritten() int64
	// IsLimitReached returns true if a hard limit has been reached.
	IsLimitReached() bool
	// LimitReason returns the reason why capture stopped (if a limit was reached).
	LimitReason() string
}

// DefaultSnaplen is the snaplen used when the caller passes 0: a full
// Ethernet frame including jumbo frames, i.e. no truncation in practice.
const DefaultSnaplen uint32 = 65535

// MaxCaptureDurationSeconds is the hard upper bound this writer accepts for
// maxDurationSeconds. The sidecar is its own trust boundary — the API tier
// caps requests far lower (1..3600) — and without a bound a caller-supplied
// value large enough to overflow time.Duration (>= ~9.2e9 seconds) turns into
// a negative duration, which makes timers fire immediately and ends the
// capture the instant it starts. 604800s (7 days) is the ratified retention
// maximum, so nothing legitimate needs more.
const MaxCaptureDurationSeconds int64 = 604800

// epbFixedOverheadBytes is the on-disk cost of one PCAPNG Enhanced Packet
// Block excluding the packet data and its 4-byte alignment padding: a 28-byte
// header (block type, block length, interface id, timestamp high/low, captured
// length, original length) plus the 4-byte trailing block-length field.
// Verified against github.com/gopacket/gopacket@v1.6.1/pcapgo/ngwrite.go
// (*NgWriter).WritePacketWithOptions, which writes w.buf[:28], the data, the
// padding, then a 4-byte trailer.
const epbFixedOverheadBytes int64 = 32

// Limit reasons reported through LimitReason once a hard limit stops a capture.
const (
	LimitReasonDurationReached = "max_duration_reached"
	LimitReasonSizeReached     = "max_size_reached"
	LimitReasonDiskFull        = "disk_full"
	LimitReasonUserRequested   = "user_requested"
)

// ErrLimitReached is returned by WritePacket as soon as a hard limit is hit —
// including for the very packet that reaches the size limit, which *is*
// written to the file. Callers must treat it as "stop capturing now" rather
// than as a lost packet: returning it from the read loop is what stops a
// capture promptly instead of leaving it nominally running until the next
// packet arrives (which may never happen if traffic goes quiet). Match it with
// errors.Is; LimitReason reports why.
var ErrLimitReached = errors.New("capture limit reached")

// countingWriter counts the bytes handed to the underlying capture file so the
// writer can enforce maxSizeBytes against real on-disk size rather than
// against packet payload bytes, which ignore per-packet block overhead.
type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	if err != nil {
		return n, fmt.Errorf("write capture file: %w", err)
	}
	return n, nil
}

// Writer writes captured packets to a PCAPNG file and enforces
// duration/size hard limits. Once either limit is reached, the writer
// stops accepting packets and finalizes the file.
type Writer struct {
	file               *os.File
	counter            *countingWriter
	writer             *pcapgo.NgWriter
	snaplen            uint32
	maxSizeBytes       int64
	maxDurationSeconds int64
	startTime          time.Time

	mu             sync.Mutex
	closed         bool
	bytesWritten   int64
	packetsWritten int64
	limitReached   bool
	limitReason    string
}

// NewWriter creates a new PCAPNG writer that records packets to the specified
// file with the given hard limits on duration and file size.
//
// snaplen is the maximum captured packet size (0 means DefaultSnaplen); every
// packet is truncated to it before being written. filterExpr is the BPF filter
// expression the capture runs with: it is recorded in the file's single
// interface block, so the file documents which traffic it does and does not
// contain. It may be empty when no filter is in force.
func NewWriter(filePath string, maxDurationSeconds int64, maxSizeBytes int64, snaplen uint32, filterExpr string) (*Writer, error) {
	if maxDurationSeconds <= 0 {
		return nil, fmt.Errorf("maxDurationSeconds must be positive, got %d", maxDurationSeconds)
	}
	if maxDurationSeconds > MaxCaptureDurationSeconds {
		return nil, fmt.Errorf("maxDurationSeconds must be at most %d, got %d", MaxCaptureDurationSeconds, maxDurationSeconds)
	}
	if maxSizeBytes <= 0 {
		return nil, fmt.Errorf("maxSizeBytes must be positive, got %d", maxSizeBytes)
	}
	if snaplen == 0 {
		snaplen = DefaultSnaplen
	}

	// Create or truncate the capture file. filePath's only production caller
	// (httpserver.Server.captureFilePath) already builds it from a
	// regex-validated capture id and re-verifies containment under its
	// capture directory before this is ever reached (see that method's doc
	// comment) - so there is no untrusted input reaching this open. Clean it
	// again here regardless: NewWriter is this package's exported entry
	// point, so it must not assume every future caller repeats that
	// containment check, and a cleaned path also can't carry a redundant
	// ".." that would otherwise resolve outside whatever directory the
	// caller intended.
	file, err := os.Create(filepath.Clean(filePath))
	if err != nil {
		return nil, fmt.Errorf("create capture file %s: %w", filePath, err)
	}

	// Initialize the PCAPNG writer with exactly one interface block, whose
	// metadata (name, snaplen, filter) describes the interface every packet
	// is written against. NewNgWriter would write an anonymous default
	// interface of its own and any AddInterface call would then append a
	// *second* block that no packet references, so the descriptive form is
	// the only correct one here (verified against
	// github.com/gopacket/gopacket@v1.6.1/pcapgo/ngwrite.go: NewNgWriter
	// delegates to NewNgWriterInterface, which writes the section header and
	// then adds the interface it is given).
	counter := &countingWriter{w: file}
	iface := pcapgo.NgInterface{
		Name:       "eth0",
		Filter:     filterExpr,
		LinkType:   layers.LinkTypeEthernet,
		SnapLength: snaplen,
	}
	ngWriter, err := pcapgo.NewNgWriterInterface(counter, iface, pcapgo.DefaultNgWriterOptions)
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("initialize PCAPNG writer: %w", err)
	}

	// Flush the section and interface blocks now so the counter reflects the
	// exact size of the file header, which counts against maxSizeBytes just
	// like packet data does.
	if err := ngWriter.Flush(); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("flush PCAPNG header: %w", err)
	}

	return &Writer{
		file:               file,
		counter:            counter,
		writer:             ngWriter,
		snaplen:            snaplen,
		maxSizeBytes:       maxSizeBytes,
		maxDurationSeconds: maxDurationSeconds,
		startTime:          time.Now(),
		bytesWritten:       counter.n,
	}, nil
}

// WritePacket writes a single packet to the PCAPNG file, truncated to the
// configured snaplen.
//
// It returns an error wrapping ErrLimitReached once a hard limit is reached.
// For the duration limit and for any call after a limit was already reached,
// the packet is rejected without being written. For the size limit, the packet
// that crosses the limit *is* written first — the error says "stop", not "this
// packet was dropped".
func (w *Writer) WritePacket(pkt *RawPacket) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	// Check if a limit has already been reached.
	if w.limitReached {
		return fmt.Errorf("capture stopped: %s: %w", w.limitReason, ErrLimitReached)
	}

	// Check duration limit (hard stop at or past the limit).
	elapsed := time.Since(w.startTime).Seconds()
	if elapsed >= float64(w.maxDurationSeconds) {
		w.limitReached = true
		w.limitReason = LimitReasonDurationReached
		return fmt.Errorf("capture stopped: %s: %w", w.limitReason, ErrLimitReached)
	}

	// Truncate packet to snaplen if necessary.
	packetData := pkt.Data
	if int64(len(packetData)) > int64(w.snaplen) {
		packetData = packetData[:w.snaplen]
	}

	// Length must be the length of the frame on the wire, before both the
	// kernel's snaplen truncation and ours. The source reports it on the
	// packet; fall back to what we actually hold when it is unset, which
	// keeps the CaptureLength <= Length invariant pcapgo enforces.
	originalLength := pkt.OriginalLength
	if originalLength < len(pkt.Data) {
		originalLength = len(pkt.Data)
	}

	ci := gopacket.CaptureInfo{
		Timestamp:      pkt.Timestamp,
		CaptureLength:  len(packetData),
		Length:         originalLength,
		InterfaceIndex: 0,
	}

	if err := w.writer.WritePacket(ci, packetData); err != nil {
		// Check if this is a disk-full error (ENOSPC).
		if isENOSPC(err) {
			w.limitReached = true
			w.limitReason = LimitReasonDiskFull
			return fmt.Errorf("disk full during capture: %w", err)
		}
		return fmt.Errorf("write packet to PCAPNG: %w", err)
	}

	// Account for the real on-disk cost of this packet: the Enhanced Packet
	// Block header and trailer plus the data padded to a 4-byte boundary.
	// Counting payload alone under-reports by ~32 bytes per packet, which for
	// small game packets overruns maxSizeBytes by tens of percent.
	padding := int64((4 - len(packetData)&3) & 3)
	w.bytesWritten += epbFixedOverheadBytes + int64(len(packetData)) + padding
	w.packetsWritten++

	// Check size limit (hard stop at or past the limit). The packet above was
	// written; returning ErrLimitReached stops the capture immediately rather
	// than waiting for a subsequent packet that may never arrive.
	if w.bytesWritten >= w.maxSizeBytes {
		w.limitReached = true
		w.limitReason = LimitReasonSizeReached
		return fmt.Errorf("capture stopped: %s: %w", w.limitReason, ErrLimitReached)
	}

	return nil
}

// Close finalizes the PCAPNG file and closes the underlying file handle.
// This must always produce a valid, complete PCAPNG file, even if called
// mid-capture (e.g., when a hard limit is hit).
//
// The PCAPNG writer buffers, so a full disk usually surfaces here rather than
// from WritePacket; a flush that fails with ENOSPC is therefore recorded as
// the disk_full limit reason as well as returned, so callers report a
// truncated capture as truncated instead of as a clean completion.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}
	w.closed = true

	// The NgWriter.Flush() writes any remaining data.
	// The file already has a valid PCAPNG structure with sections and interface blocks,
	// so closing the writer produces a complete, valid file.
	if err := w.writer.Flush(); err != nil {
		if isENOSPC(err) {
			w.limitReached = true
			w.limitReason = LimitReasonDiskFull
		}
		_ = w.file.Close()
		return fmt.Errorf("flush PCAPNG writer: %w", err)
	}

	// Everything the writer buffered has now reached the file, so the counter
	// is the authoritative file size.
	w.bytesWritten = w.counter.n

	if err := w.file.Close(); err != nil {
		return fmt.Errorf("close capture file: %w", err)
	}

	return nil
}

// BytesWritten returns the size of the capture file on disk: the PCAPNG
// header blocks plus every packet block written so far.
func (w *Writer) BytesWritten() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.bytesWritten
}

// PacketsWritten returns the cumulative number of packets written to the file.
func (w *Writer) PacketsWritten() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.packetsWritten
}

// IsLimitReached returns true if a hard limit has been reached.
func (w *Writer) IsLimitReached() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.limitReached
}

// LimitReason returns the reason why capture stopped (if a limit was reached).
// Returns an empty string if no limit has been reached.
func (w *Writer) LimitReason() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.limitReason
}

// isENOSPC checks if an error is a disk-full (ENOSPC) error. Real write
// errors from *os.File/*pcapgo.NgWriter wrap the underlying syscall error
// (typically with a path prefix via *fs.PathError), so this walks the error
// chain with errors.Is rather than comparing error strings, which never
// match once the error has been wrapped.
func isENOSPC(err error) bool {
	return errors.Is(err, syscall.ENOSPC)
}
