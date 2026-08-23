package capture

import (
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/gopacket/gopacket/pcapgo"
)

const testFilter = "udp port 25565"

// TestWriter_NormalStop tests that a normal stop produces a valid file
// openable via gopacket's own reader, and that counters are accurate.
func TestWriter_NormalStop(t *testing.T) {
	// Create a temporary file for the test.
	tmpFile := t.TempDir() + "/test_normal_stop.pcapng"

	// Create a writer with generous limits.
	w, err := NewWriter(tmpFile, 300, 1000000, 65535, testFilter)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer w.Close()

	// Write a few test packets.
	testPackets := []*RawPacket{
		{
			Data:      []byte{0x45, 0x00, 0x00, 0x28},
			Timestamp: time.Now(),
		},
		{
			Data:      []byte{0x45, 0x00, 0x00, 0x30, 0x00, 0x01},
			Timestamp: time.Now().Add(1 * time.Millisecond),
		},
		{
			Data:      []byte{0x45, 0x00, 0x00, 0x20},
			Timestamp: time.Now().Add(2 * time.Millisecond),
		},
	}

	for _, pkt := range testPackets {
		if err := w.WritePacket(pkt); err != nil {
			t.Fatalf("WritePacket failed: %v", err)
		}
	}

	// Verify counters.
	if w.PacketsWritten() != int64(len(testPackets)) {
		t.Errorf("PacketsWritten() = %d; want %d", w.PacketsWritten(), len(testPackets))
	}

	payloadBytes := int64(len(testPackets[0].Data) + len(testPackets[1].Data) + len(testPackets[2].Data))
	if w.BytesWritten() <= payloadBytes {
		t.Errorf("BytesWritten() = %d; want more than the %d payload bytes (file overhead must count)", w.BytesWritten(), payloadBytes)
	}

	// Close and verify the file is valid PCAPNG.
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// BytesWritten is the file's real size, not a payload total.
	info, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("stat capture file: %v", err)
	}
	if info.Size() != w.BytesWritten() {
		t.Errorf("BytesWritten() = %d; want the on-disk size %d", w.BytesWritten(), info.Size())
	}

	// Read back the file using gopacket's reader.
	f, err := os.Open(tmpFile)
	if err != nil {
		t.Fatalf("open file for reading: %v", err)
	}
	defer f.Close()

	reader, err := pcapgo.NewNgReader(f, pcapgo.DefaultNgReaderOptions)
	if err != nil {
		t.Fatalf("NewNgReader failed: %v", err)
	}

	// Count packets read back.
	packetsRead := 0
	for {
		data, _, err := reader.ReadPacketData()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("ReadPacketData failed: %v", err)
		}
		if len(data) > 0 {
			packetsRead++
		}
	}

	if packetsRead != len(testPackets) {
		t.Errorf("read back %d packets; want %d", packetsRead, len(testPackets))
	}
}

// TestWriter_SingleInterfaceBlock verifies the file describes exactly one
// interface and that every packet belongs to it. A second, anonymous interface
// block (which is what an extra AddInterface call produces) would leave the
// packets pointing at metadata that describes nothing.
func TestWriter_SingleInterfaceBlock(t *testing.T) {
	tmpFile := t.TempDir() + "/test_interface_block.pcapng"

	const snaplen uint32 = 1024

	w, err := NewWriter(tmpFile, 300, 1000000, snaplen, testFilter)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	if err := w.WritePacket(&RawPacket{Data: []byte{0x01, 0x02, 0x03}, Timestamp: time.Now()}); err != nil {
		t.Fatalf("WritePacket failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	f, err := os.Open(tmpFile)
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer f.Close()

	reader, err := pcapgo.NewNgReader(f, pcapgo.DefaultNgReaderOptions)
	if err != nil {
		t.Fatalf("NewNgReader failed: %v", err)
	}
	if _, _, err := reader.ReadPacketData(); err != nil {
		t.Fatalf("ReadPacketData failed: %v", err)
	}

	if reader.NInterfaces() != 1 {
		t.Fatalf("file has %d interface blocks; want exactly 1", reader.NInterfaces())
	}

	iface, err := reader.Interface(0)
	if err != nil {
		t.Fatalf("Interface(0) failed: %v", err)
	}
	if iface.Name != "eth0" {
		t.Errorf("interface name = %q; want %q", iface.Name, "eth0")
	}
	if iface.SnapLength != snaplen {
		t.Errorf("interface snaplen = %d; want %d", iface.SnapLength, snaplen)
	}
	if iface.Filter != testFilter {
		t.Errorf("interface filter = %q; want %q", iface.Filter, testFilter)
	}
}

// TestWriter_RecordsWireLengthAndSnaplen verifies that a truncated packet
// records how long the frame was on the wire, not how much of it was kept.
func TestWriter_RecordsWireLengthAndSnaplen(t *testing.T) {
	tmpFile := t.TempDir() + "/test_wire_length.pcapng"

	const snaplen uint32 = 16

	w, err := NewWriter(tmpFile, 300, 1000000, snaplen, testFilter)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	pkt := &RawPacket{
		Data:           make([]byte, 64),
		Timestamp:      time.Now(),
		OriginalLength: 1500,
	}
	if err := w.WritePacket(pkt); err != nil {
		t.Fatalf("WritePacket failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	f, err := os.Open(tmpFile)
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer f.Close()

	reader, err := pcapgo.NewNgReader(f, pcapgo.DefaultNgReaderOptions)
	if err != nil {
		t.Fatalf("NewNgReader failed: %v", err)
	}

	data, ci, err := reader.ReadPacketData()
	if err != nil {
		t.Fatalf("ReadPacketData failed: %v", err)
	}
	if len(data) != int(snaplen) {
		t.Errorf("captured %d bytes; want the snaplen %d", len(data), snaplen)
	}
	if ci.CaptureLength != int(snaplen) {
		t.Errorf("CaptureLength = %d; want %d", ci.CaptureLength, snaplen)
	}
	if ci.Length != 1500 {
		t.Errorf("Length = %d; want the wire length 1500", ci.Length)
	}
}

// TestWriter_WireLengthFallsBackToPayload verifies that a source which does
// not report an original length still yields a self-consistent file.
func TestWriter_WireLengthFallsBackToPayload(t *testing.T) {
	tmpFile := t.TempDir() + "/test_wire_length_fallback.pcapng"

	w, err := NewWriter(tmpFile, 300, 1000000, 65535, testFilter)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	if err := w.WritePacket(&RawPacket{Data: make([]byte, 20), Timestamp: time.Now()}); err != nil {
		t.Fatalf("WritePacket failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	f, err := os.Open(tmpFile)
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer f.Close()

	reader, err := pcapgo.NewNgReader(f, pcapgo.DefaultNgReaderOptions)
	if err != nil {
		t.Fatalf("NewNgReader failed: %v", err)
	}

	_, ci, err := reader.ReadPacketData()
	if err != nil {
		t.Fatalf("ReadPacketData failed: %v", err)
	}
	if ci.Length != 20 || ci.CaptureLength != 20 {
		t.Errorf("CaptureLength/Length = %d/%d; want 20/20", ci.CaptureLength, ci.Length)
	}
}

// TestWriter_SizeLimitCountsFileOverhead tests that the size limit is enforced
// against the real on-disk size — header blocks and per-packet block overhead
// included — and that hitting it stops the capture on the spot rather than on
// the next packet, which may never arrive.
func TestWriter_SizeLimitCountsFileOverhead(t *testing.T) {
	tmpFile := t.TempDir() + "/test_size_limit.pcapng"

	const maxSize int64 = 4096
	const payloadSize = 40

	w, err := NewWriter(tmpFile, 300, maxSize, 65535, testFilter)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer w.Close()

	headerBytes := w.BytesWritten()
	if headerBytes <= 0 {
		t.Fatalf("BytesWritten() = %d before any packet; want the PCAPNG header size", headerBytes)
	}

	testPacket := &RawPacket{
		Data:      make([]byte, payloadSize),
		Timestamp: time.Now(),
	}

	written := int64(0)
	for i := 0; i < 1000; i++ {
		err := w.WritePacket(testPacket)
		if err != nil {
			if !errors.Is(err, ErrLimitReached) {
				t.Fatalf("WritePacket failed unexpectedly: %v", err)
			}
			// The packet that crosses the limit is still written.
			written++
			break
		}
		written++
	}

	if written == 0 {
		t.Fatal("no packets were written before the size limit")
	}
	if !w.IsLimitReached() {
		t.Fatal("size limit was not reached")
	}
	if w.LimitReason() != LimitReasonSizeReached {
		t.Errorf("LimitReason() = %q; want %q", w.LimitReason(), LimitReasonSizeReached)
	}

	// Each packet costs its payload plus the enhanced-packet-block overhead;
	// payloadSize is 4-byte aligned, so there is no padding to account for.
	wantBytes := headerBytes + written*(epbFixedOverheadBytes+payloadSize)
	if w.BytesWritten() != wantBytes {
		t.Errorf("BytesWritten() = %d; want %d (%d packets plus %d header bytes)", w.BytesWritten(), wantBytes, written, headerBytes)
	}
	if w.BytesWritten() < maxSize {
		t.Errorf("BytesWritten() = %d; want at least the limit %d", w.BytesWritten(), maxSize)
	}
	if payloadOnly := written * payloadSize; payloadOnly >= maxSize {
		t.Errorf("payload alone (%d bytes) already reached the limit; the test cannot show overhead is counted", payloadOnly)
	}

	// A further packet is rejected outright.
	err = w.WritePacket(testPacket)
	if !errors.Is(err, ErrLimitReached) {
		t.Errorf("WritePacket after the limit = %v; want ErrLimitReached", err)
	}

	// Close and verify the file is valid and no larger than we reported.
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	info, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("stat capture file: %v", err)
	}
	if info.Size() != wantBytes {
		t.Errorf("on-disk size = %d; want the accounted %d", info.Size(), wantBytes)
	}

	f, err := os.Open(tmpFile)
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer f.Close()

	if _, err := pcapgo.NewNgReader(f, pcapgo.DefaultNgReaderOptions); err != nil {
		t.Fatalf("NewNgReader failed: %v", err)
	}
}

// TestWriter_DurationLimitStop tests that a duration limit triggers a clean stop
// and produces a valid file.
// Note: this test uses a short duration for speed, but the actual check in WritePacket
// uses real time.Since(), so we need to wait or simulate time passage.
func TestWriter_DurationLimitStop(t *testing.T) {
	tmpFile := t.TempDir() + "/test_duration_limit.pcapng"

	// Create a writer with a very short duration limit (1 second).
	// We'll use a generous size limit so size doesn't trigger first.
	w, err := NewWriter(tmpFile, 1, 1000000, 65535, testFilter)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer w.Close()

	// Write one packet immediately.
	testPacket := &RawPacket{
		Data:      []byte{0x45, 0x00, 0x00, 0x28},
		Timestamp: time.Now(),
	}

	if err := w.WritePacket(testPacket); err != nil {
		t.Fatalf("WritePacket failed: %v", err)
	}

	// Wait for the duration limit to expire.
	time.Sleep(1100 * time.Millisecond)

	// Try to write another packet; it should be rejected.
	err = w.WritePacket(testPacket)
	if !errors.Is(err, ErrLimitReached) {
		t.Fatalf("WritePacket after the duration limit = %v; want ErrLimitReached", err)
	}

	if !w.IsLimitReached() {
		t.Fatal("duration limit was not reached")
	}

	if w.LimitReason() != LimitReasonDurationReached {
		t.Errorf("LimitReason() = %q; want %q", w.LimitReason(), LimitReasonDurationReached)
	}

	// Close and verify the file is valid.
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	f, err := os.Open(tmpFile)
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer f.Close()

	_, err = pcapgo.NewNgReader(f, pcapgo.DefaultNgReaderOptions)
	if err != nil {
		t.Fatalf("NewNgReader failed: %v", err)
	}
}

// TestWriter_RejectsInvalidLimits tests that limits outside the accepted range
// are refused at construction. An unbounded duration is the dangerous case: a
// value large enough to overflow time.Duration turns into a negative one,
// which would end a capture the instant it started.
func TestWriter_RejectsInvalidLimits(t *testing.T) {
	tests := []struct {
		name               string
		maxDurationSeconds int64
		maxSizeBytes       int64
	}{
		{"zero duration", 0, 1000000},
		{"negative duration", -1, 1000000},
		{"duration above maximum", MaxCaptureDurationSeconds + 1, 1000000},
		{"duration that overflows a time.Duration", 1 << 62, 1000000},
		{"zero size", 300, 0},
		{"negative size", 300, -1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpFile := t.TempDir() + "/test_invalid_limits.pcapng"
			w, err := NewWriter(tmpFile, tc.maxDurationSeconds, tc.maxSizeBytes, 65535, testFilter)
			if err == nil {
				_ = w.Close()
				t.Fatal("NewWriter succeeded; want an error")
			}
			if w != nil {
				t.Error("NewWriter returned a non-nil Writer with an error")
			}
			if _, statErr := os.Stat(tmpFile); statErr == nil {
				t.Error("NewWriter left a capture file behind after rejecting its limits")
			}
		})
	}

	// The documented maximum itself must be accepted.
	tmpFile := t.TempDir() + "/test_max_duration.pcapng"
	w, err := NewWriter(tmpFile, MaxCaptureDurationSeconds, 1000000, 65535, testFilter)
	if err != nil {
		t.Fatalf("NewWriter with the maximum duration failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

// TestWriter_MidCaptureStop tests that stopping mid-capture (without hitting limits)
// still produces a valid file with correct counters.
func TestWriter_MidCaptureStop(t *testing.T) {
	tmpFile := t.TempDir() + "/test_mid_stop.pcapng"

	w, err := NewWriter(tmpFile, 300, 1000000, 65535, testFilter)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	// Write some packets.
	for i := 0; i < 5; i++ {
		pkt := &RawPacket{
			Data:      []byte{byte(i), 0x45, 0x00},
			Timestamp: time.Now(),
		}
		if err := w.WritePacket(pkt); err != nil {
			t.Fatalf("WritePacket failed: %v", err)
		}
	}

	// Immediately close without hitting any limits.
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify the file is valid and readable.
	f, err := os.Open(tmpFile)
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer f.Close()

	reader, err := pcapgo.NewNgReader(f, pcapgo.DefaultNgReaderOptions)
	if err != nil {
		t.Fatalf("NewNgReader failed: %v", err)
	}

	packetsRead := 0
	for {
		data, _, err := reader.ReadPacketData()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("ReadPacketData failed: %v", err)
		}
		if len(data) > 0 {
			packetsRead++
		}
	}

	if packetsRead != 5 {
		t.Errorf("read back %d packets; want 5", packetsRead)
	}
}

// TestWriter_PacketCounterAccuracy tests that packet and byte counters
// accurately reflect what was written, including per-packet block overhead
// and the padding that aligns each block to a 4-byte boundary.
func TestWriter_PacketCounterAccuracy(t *testing.T) {
	tmpFile := t.TempDir() + "/test_counters.pcapng"

	w, err := NewWriter(tmpFile, 300, 1000000, 65535, testFilter)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer w.Close()

	testCases := []struct {
		data []byte
		want int64
	}{
		{[]byte{0x01, 0x02, 0x03}, epbFixedOverheadBytes + 3 + 1},
		{[]byte{0x04, 0x05, 0x06, 0x07}, epbFixedOverheadBytes + 4},
		{make([]byte, 100), epbFixedOverheadBytes + 100},
	}

	totalBytes := w.BytesWritten()
	totalPackets := int64(0)

	for i, tc := range testCases {
		pkt := &RawPacket{
			Data:      tc.data,
			Timestamp: time.Now(),
		}
		if err := w.WritePacket(pkt); err != nil {
			t.Fatalf("WritePacket %d failed: %v", i, err)
		}
		totalBytes += tc.want
		totalPackets++

		if w.PacketsWritten() != totalPackets {
			t.Errorf("after packet %d: PacketsWritten() = %d; want %d", i, w.PacketsWritten(), totalPackets)
		}
		if w.BytesWritten() != totalBytes {
			t.Errorf("after packet %d: BytesWritten() = %d; want %d", i, w.BytesWritten(), totalBytes)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	info, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("stat capture file: %v", err)
	}
	if info.Size() != totalBytes {
		t.Errorf("on-disk size = %d; want the accounted %d", info.Size(), totalBytes)
	}
}

// TestWriter_CloseIdempotent tests that Close() can be called multiple times
// without error (idempotent behavior).
func TestWriter_CloseIdempotent(t *testing.T) {
	tmpFile := t.TempDir() + "/test_close_idempotent.pcapng"

	w, err := NewWriter(tmpFile, 300, 1000000, 65535, testFilter)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	// Close multiple times.
	for i := 0; i < 3; i++ {
		if err := w.Close(); err != nil {
			t.Fatalf("Close call %d failed: %v", i, err)
		}
	}
}

// TestWriter_WriteAfterClose tests that writing after Close() returns an error.
func TestWriter_WriteAfterClose(t *testing.T) {
	tmpFile := t.TempDir() + "/test_write_after_close.pcapng"

	w, err := NewWriter(tmpFile, 300, 1000000, 65535, testFilter)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	pkt := &RawPacket{
		Data:      []byte{0x01, 0x02},
		Timestamp: time.Now(),
	}

	err = w.WritePacket(pkt)
	if err == nil {
		t.Fatal("WritePacket after Close should return an error")
	}
}
