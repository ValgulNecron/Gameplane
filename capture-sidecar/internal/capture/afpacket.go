// Package capture provides packet capture via AF_PACKET with BPF filtering,
// PCAPNG file writing, and capture lifecycle management.
package capture

import "context"

// Capturer manages AF_PACKET socket setup and live packet capture.
// It applies BPF filters at the kernel level and reads packets from an
// mmap'd TPacket buffer for efficiency.
type Capturer struct {
	// TODO: add fields for AF_PACKET socket, mmap'd buffer, BPF filter handle
}

// Start begins capturing packets matching the configured filter.
// It returns an error if the AF_PACKET socket cannot be created or
// the filter cannot be loaded into the kernel.
func (c *Capturer) Start(ctx context.Context) error {
	// TODO: implement AF_PACKET setup, mmap, BPF loading
	return nil
}

// Stop closes the AF_PACKET socket and stops packet capture.
func (c *Capturer) Stop() error {
	// TODO: implement socket cleanup
	return nil
}

// PacketChan returns a channel that yields captured packets.
// Callers must read from this channel; packets are dropped if the
// receiver falls behind.
func (c *Capturer) PacketChan() <-chan Packet {
	// TODO: implement packet delivery channel
	return nil
}

// Packet represents a single captured network packet.
type Packet struct {
	// TODO: add timestamp, data, length fields
}
