package rcon

import (
	"encoding/binary"
	"net"
	"testing"
)

// TestExecWriteFailsOnClosedPipe deterministically drives Exec's
// writePacket-failure path (as opposed to the racy "write vs RST"
// scenario the timing-dependent tests describe): c.conn is a net.Pipe
// end whose peer has already been closed, so the very first Write
// inside writePacket returns io.ErrClosedPipe synchronously, no race
// involved.
func TestExecWriteFailsOnClosedPipe(t *testing.T) {
	c := New("ignored", 0, func() (string, error) { return "pw", nil })
	a, b := net.Pipe()
	if err := b.Close(); err != nil {
		t.Fatalf("close pipe peer: %v", err)
	}
	c.conn = a
	c.execDeadline = 0 // also exercises the <=0 fallback to defaultExecDeadline

	_, err := c.Exec("status")
	if err == nil {
		t.Fatal("Exec on a pipe whose peer is already closed should fail")
	}
	if c.conn != nil {
		t.Error("a failed write must drop the connection (c.conn should be nil)")
	}
}

// TestReadPacketFailsOnShortPipe drives readPacket's body-read error path
// deterministically: the header announces a legal-sized body (20 bytes,
// within [minPacketSize, maxPacketSize]) but the peer closes before
// sending it, so io.ReadFull on the body fails.
func TestReadPacketFailsOnShortPipe(t *testing.T) {
	c := New("ignored", 0, func() (string, error) { return "pw", nil })
	a, b := net.Pipe()
	c.conn = a

	done := make(chan struct{})
	go func() {
		defer close(done)
		hdr := make([]byte, 4)
		binary.LittleEndian.PutUint32(hdr, 20)
		_, _ = b.Write(hdr)
		_ = b.Close()
	}()

	_, _, _, err := c.readPacket()
	<-done
	if err == nil {
		t.Fatal("readPacket should fail when the peer closes before sending the full body")
	}
}
