package capture

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/afpacket"
	"github.com/gopacket/gopacket/layers"
	"golang.org/x/net/bpf"
)

type queuedFrame struct {
	data []byte
	info gopacket.CaptureInfo
	err  error
}

// queuedPacketHandle models frames already resident in the packet ring when
// the kernel filter is attached. Attaching BPF cannot remove these frames.
type queuedPacketHandle struct {
	frames    []queuedFrame
	reads     int
	closes    int
	afterRead func()
}

func (h *queuedPacketHandle) ReadPacketData() ([]byte, gopacket.CaptureInfo, error) {
	h.reads++
	if h.afterRead != nil {
		h.afterRead()
	}
	if len(h.frames) == 0 {
		return nil, gopacket.CaptureInfo{}, io.EOF
	}
	frame := h.frames[0]
	h.frames = h.frames[1:]
	return frame.data, frame.info, frame.err
}

func (h *queuedPacketHandle) Close() { h.closes++ }

func captureTCPFrame(t *testing.T, source, destination layers.TCPPort) []byte {
	t.Helper()
	ethernet := &layers.Ethernet{
		SrcMAC: net.HardwareAddr{0, 1, 2, 3, 4, 5}, DstMAC: net.HardwareAddr{6, 7, 8, 9, 10, 11},
		EthernetType: layers.EthernetTypeIPv4,
	}
	ip := &layers.IPv4{Version: 4, IHL: 5, TTL: 64, Protocol: layers.IPProtocolTCP,
		SrcIP: net.IPv4(10, 0, 0, 1), DstIP: net.IPv4(10, 0, 0, 2)}
	tcp := &layers.TCP{SrcPort: source, DstPort: destination, SYN: true, Seq: 1, Window: 1024}
	if err := tcp.SetNetworkLayerForChecksum(ip); err != nil {
		t.Fatal(err)
	}
	buffer := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(buffer, gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}, ethernet, ip, tcp); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func captureFilterVM(t *testing.T) *bpf.VM {
	t.Helper()
	filter, err := CompileFilter("tcp port 12345")
	if err != nil {
		t.Fatal(err)
	}
	vm, err := bpf.NewVM(filter.Instructions())
	if err != nil {
		t.Fatal(err)
	}
	return vm
}

func TestAFPacketSourceRejectsQueuedControlTrafficBeforeOutput(t *testing.T) {
	for _, test := range []struct {
		name        string
		source      layers.TCPPort
		destination layers.TCPPort
		snaplen     uint32
	}{
		{"inbound game frame", 54606, 12345, DefaultSnaplen},
		{"outbound game frame", 12345, 54606, DefaultSnaplen},
		{"filter before short snaplen", 54606, 12345, 14},
	} {
		t.Run(test.name, func(t *testing.T) {
			control := captureTCPFrame(t, 9091, 54606)
			game := captureTCPFrame(t, test.source, test.destination)
			timestamp := time.Unix(1_700_000_000, 123456789)
			handle := &queuedPacketHandle{frames: []queuedFrame{
				{data: control, info: gopacket.CaptureInfo{Timestamp: timestamp.Add(-time.Second), Length: len(control)}},
				{data: game, info: gopacket.CaptureInfo{Timestamp: timestamp, Length: len(game)}},
			}}
			source := &AFPacketSource{handle: handle, filterVM: captureFilterVM(t), snaplen: test.snaplen}
			packet, err := source.ReadPacket(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			want := game
			if int(test.snaplen) < len(want) {
				want = want[:test.snaplen]
			}
			if handle.reads != 2 || !bytes.Equal(packet.Data, want) || packet.OriginalLength != len(game) || !packet.Timestamp.Equal(timestamp) {
				t.Fatalf("queued control packet escaped or accepted metadata changed: reads=%d packet=%+v", handle.reads, packet)
			}
		})
	}
}

func TestAFPacketSourceCancellationWhileDiscarding(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	handle := &queuedPacketHandle{frames: []queuedFrame{
		{data: captureTCPFrame(t, 9091, 54606)},
		{data: captureTCPFrame(t, 12345, 54606)},
	}, afterRead: cancel}
	source := &AFPacketSource{handle: handle, filterVM: captureFilterVM(t)}
	packet, err := source.ReadPacket(ctx)
	if packet != nil || !errors.Is(err, context.Canceled) || handle.reads != 1 {
		t.Fatalf("discard loop ignored cancellation: packet=%v err=%v reads=%d", packet, err, handle.reads)
	}
}

func TestAFPacketSourceTimeoutThenHandleError(t *testing.T) {
	readErr := errors.New("capture socket failed")
	handle := &queuedPacketHandle{frames: []queuedFrame{{err: afpacket.ErrTimeout}, {err: readErr}}}
	source := &AFPacketSource{handle: handle, filterVM: captureFilterVM(t)}
	packet, err := source.ReadPacket(t.Context())
	if packet != nil || !errors.Is(err, readErr) || handle.reads != 2 {
		t.Fatalf("socket error was not preserved: packet=%v err=%v reads=%d", packet, err, handle.reads)
	}
}

func TestAFPacketSourceVMErrorFailsClosed(t *testing.T) {
	// NegateA assembles successfully but is not implemented by x/net/bpf's
	// VM. Use that real VM error path rather than an artificial filter hook.
	vm, err := bpf.NewVM([]bpf.Instruction{bpf.NegateA{}, bpf.RetConstant{Val: 65535}})
	if err != nil {
		t.Fatal(err)
	}
	handle := &queuedPacketHandle{frames: []queuedFrame{{data: captureTCPFrame(t, 12345, 54606)}}}
	source := &AFPacketSource{handle: handle, filterVM: vm}
	packet, err := source.ReadPacket(t.Context())
	if packet != nil || err == nil || !strings.Contains(err.Error(), "evaluate BPF output filter") || handle.reads != 1 {
		t.Fatalf("VM error did not fail closed: packet=%v err=%v reads=%d", packet, err, handle.reads)
	}
}

func TestAFPacketSourceInvalidFilterRejectedBeforeOpen(t *testing.T) {
	// Neither privileges nor an interface should be needed to reject syntax.
	source, err := NewAFPacketSource("no-such-interface", DefaultSnaplen, "tcp port definitely-not-a-port")
	if source != nil || err == nil || !strings.Contains(err.Error(), "compile BPF filter") {
		t.Fatalf("invalid filter reached socket creation: source=%v err=%v", source, err)
	}
}

func TestAFPacketSourceCloseIsIdempotent(t *testing.T) {
	handle := &queuedPacketHandle{}
	source := &AFPacketSource{handle: handle}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	packet, err := source.ReadPacket(t.Context())
	if handle.closes != 1 || handle.reads != 0 || packet != nil || err == nil {
		t.Fatalf("closed source touched handle: closes=%d reads=%d packet=%v err=%v", handle.closes, handle.reads, packet, err)
	}
}
