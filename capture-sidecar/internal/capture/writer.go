// Package capture provides packet capture via AF_PACKET with BPF filtering,
// PCAPNG file writing, and capture lifecycle management.
package capture

import (
	"context"
)

// Writer writes captured packets to a PCAPNG file and enforces
// duration/size limits.
type Writer struct {
	// TODO: add fields for PCAPNG writer, size/duration limit tracking
}

// WritePacket writes a single packet to the PCAPNG file.
// Returns an error if the write fails or if a hard limit is reached
// (maxSizeBytes or maxDurationSeconds).
func (w *Writer) WritePacket(ctx context.Context, pkt Packet) error {
	// TODO: implement PCAPNG write with limit checking
	return nil
}

// Close finalizes the PCAPNG file and closes the underlying file handle.
func (w *Writer) Close() error {
	// TODO: implement cleanup
	return nil
}

// Status returns the current write statistics.
func (w *Writer) Status() WriterStatus {
	// TODO: implement status gathering
	return WriterStatus{}
}

// WriterStatus holds current capture file statistics.
type WriterStatus struct {
	BytesWritten   int64
	PacketsWritten int64
}
