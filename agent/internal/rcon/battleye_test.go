package rcon

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash/crc32"
	"net"
	"strings"
	"testing"
	"time"
)

// testBuildPacket assembles a BE packet the same way a real server would,
// for use as FAKE SERVER traffic sent to the client under test. This is
// distinct from — and does not substitute for — TestBattlEyeCRCByteRange
// below, which pins what the CLIENT must put on the wire against an
// independently hand-computed expectation, not against this helper.
func testBuildPacket(t *testing.T, kind byte, payload []byte) []byte {
	t.Helper()
	inner := append([]byte{0xFF, kind}, payload...)
	crc := crc32.ChecksumIEEE(inner)
	pkt := make([]byte, 0, 6+len(inner))
	pkt = append(pkt, 'B', 'E')
	var crcBuf [4]byte
	binary.LittleEndian.PutUint32(crcBuf[:], crc)
	pkt = append(pkt, crcBuf[:]...)
	pkt = append(pkt, inner...)
	return pkt
}

// beFakeAddr splits a net.PacketConn's local UDP address into the
// host/port pair NewBattlEye expects.
func beFakeAddr(t *testing.T, pc net.PacketConn) (string, int) {
	t.Helper()
	addr, ok := pc.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("unexpected local addr type %T", pc.LocalAddr())
	}
	return addr.IP.String(), addr.Port
}

func newBEFakeServer(t *testing.T) net.PacketConn {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = pc.Close() })
	return pc
}

// shortClient builds a BattlEye client with every timeout shrunk so
// tests never wait out production-sized deadlines.
func shortClient(host string, port int, pw string) *BattlEye {
	c := NewBattlEye(host, port, func() (string, error) { return pw, nil })
	c.dialTimeout = 500 * time.Millisecond
	c.ackTimeout = 150 * time.Millisecond
	c.execDeadline = 500 * time.Millisecond
	return c
}

// ---------------------------------------------------------------------
// 1. CRC32 byte range — the most important test in this file.
// ---------------------------------------------------------------------

// TestBattlEyeCRCByteRange hand-builds the EXPECTED wire bytes for a
// login packet with password "testpass" using an independently computed
// CRC32 (Python's zlib.crc32 over 0xFF, 0x00, "testpass" — computed
// outside of, and without calling, any code in this package) and asserts
// the client puts EXACTLY those bytes on the wire. This pins the CRC's
// byte range: per the spec it covers 0xFF + type + payload, NOT the
// whole packet and NOT just the payload. A wrong byte range changes the
// resulting CRC, so an implementation with the wrong range fails this
// exact byte comparison instead of only failing against itself.
func TestBattlEyeCRCByteRange(t *testing.T) {
	const wantHex = "42451271899dff007465737470617373"
	want, err := hex.DecodeString(wantHex)
	if err != nil {
		t.Fatalf("bad test fixture: %v", err)
	}

	pc := newBEFakeServer(t)
	host, port := beFakeAddr(t, pc)

	gotCh := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 2048)
		for i := 0; i < 2; i++ {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			pkt := append([]byte(nil), buf[:n]...)
			switch i {
			case 0:
				// First packet from the client must be the login packet —
				// capture its raw bytes for the pinned comparison below.
				gotCh <- pkt
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeLogin, []byte{beLoginOK}), addr)
			case 1:
				seq := pkt[8]
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeCommand, append([]byte{seq}, []byte("ok")...)), addr)
			}
		}
	}()

	client := shortClient(host, port, "testpass")
	defer func() { _ = client.Close() }()

	if _, err := client.Exec("noop"); err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	select {
	case got := <-gotCh:
		if !bytes.Equal(got, want) {
			t.Errorf("login packet on the wire:\n got  %x\n want %x", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server never received the login packet")
	}
}

// ---------------------------------------------------------------------
// 2. Login success and failure.
// ---------------------------------------------------------------------

func TestBattlEyeLoginSuccess(t *testing.T) {
	pc := newBEFakeServer(t)
	host, port := beFakeAddr(t, pc)

	go func() {
		buf := make([]byte, 2048)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			pkt := append([]byte(nil), buf[:n]...)
			switch pkt[7] {
			case beTypeLogin:
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeLogin, []byte{beLoginOK}), addr)
			case beTypeCommand:
				seq := pkt[8]
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeCommand, append([]byte{seq}, []byte("ok")...)), addr)
			}
		}
	}()

	client := shortClient(host, port, "correct-password")
	defer func() { _ = client.Close() }()

	got, err := client.Exec("status")
	if err != nil {
		t.Fatalf("Exec failed on a server that accepts the password: %v", err)
	}
	if got != "ok" {
		t.Errorf("Exec = %q, want %q", got, "ok")
	}
}

func TestBattlEyeLoginFailure(t *testing.T) {
	pc := newBEFakeServer(t)
	host, port := beFakeAddr(t, pc)

	go func() {
		buf := make([]byte, 2048)
		_, addr, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}
		// 0x00 payload byte == rejected, per the spec.
		_, _ = pc.WriteTo(testBuildPacket(t, beTypeLogin, []byte{0x00}), addr)
	}()

	client := shortClient(host, port, "wrong-password")
	defer func() { _ = client.Close() }()

	_, err := client.Exec("status")
	if err == nil {
		t.Fatal("expected an error for a rejected password, got nil")
	}
	if !errors.Is(err, ErrAuth) {
		t.Errorf("expected errors.Is(err, ErrAuth), got %v", err)
	}
}

// ---------------------------------------------------------------------
// 3. Single-packet command reply.
// ---------------------------------------------------------------------

func TestBattlEyeSinglePacketReply(t *testing.T) {
	pc := newBEFakeServer(t)
	host, port := beFakeAddr(t, pc)

	go func() {
		buf := make([]byte, 2048)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			pkt := append([]byte(nil), buf[:n]...)
			switch pkt[7] {
			case beTypeLogin:
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeLogin, []byte{beLoginOK}), addr)
			case beTypeCommand:
				seq := pkt[8]
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeCommand, append([]byte{seq}, []byte("player1, player2")...)), addr)
			}
		}
	}()

	client := shortClient(host, port, "pw")
	defer func() { _ = client.Close() }()

	got, err := client.Exec("players")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if got != "player1, player2" {
		t.Errorf("Exec(players) = %q, want %q", got, "player1, player2")
	}
}

// ---------------------------------------------------------------------
// 4. Multi-packet reply delivered OUT OF ORDER.
// ---------------------------------------------------------------------

func TestBattlEyeMultiPacketOutOfOrder(t *testing.T) {
	pc := newBEFakeServer(t)
	host, port := beFakeAddr(t, pc)

	go func() {
		buf := make([]byte, 2048)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			pkt := append([]byte(nil), buf[:n]...)
			switch pkt[7] {
			case beTypeLogin:
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeLogin, []byte{beLoginOK}), addr)
			case beTypeCommand:
				seq := pkt[8]
				// Deliberately send page 1 BEFORE page 0 — the client must
				// reassemble by index, not by arrival order.
				page1 := append([]byte{seq, 0x00, 2, 1}, []byte("World!")...)
				page0 := append([]byte{seq, 0x00, 2, 0}, []byte("Hello, ")...)
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeCommand, page1), addr)
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeCommand, page0), addr)
			}
		}
	}()

	client := shortClient(host, port, "pw")
	defer func() { _ = client.Close() }()

	got, err := client.Exec("bigcommand")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if got != "Hello, World!" {
		t.Errorf("Exec = %q, want %q (index-ordered, not arrival-ordered)", got, "Hello, World!")
	}
}

// ---------------------------------------------------------------------
// 5. Server message (0x02) is acked with the same sequence byte.
// ---------------------------------------------------------------------

func TestBattlEyeServerMessageAcked(t *testing.T) {
	pc := newBEFakeServer(t)
	host, port := beFakeAddr(t, pc)

	const pushedSeq = 42
	ackCh := make(chan []byte, 1)

	go func() {
		buf := make([]byte, 2048)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			pkt := append([]byte(nil), buf[:n]...)
			switch pkt[7] {
			case beTypeLogin:
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeLogin, []byte{beLoginOK}), addr)
				// Push an unsolicited server message right after login —
				// a real server would send this whenever it wants,
				// independent of any command.
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeMessage, append([]byte{pushedSeq}, []byte("chat: hi")...)), addr)
			case beTypeCommand:
				seq := pkt[8]
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeCommand, append([]byte{seq}, []byte("ok")...)), addr)
			case beTypeMessage:
				// This is the client's ack of our pushed message.
				select {
				case ackCh <- pkt:
				default:
				}
			}
		}
	}()

	client := shortClient(host, port, "pw")
	defer func() { _ = client.Close() }()

	if _, err := client.Exec("noop"); err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	select {
	case got := <-ackCh:
		want := testBuildPacket(t, beTypeMessage, []byte{pushedSeq})
		if !bytes.Equal(got, want) {
			t.Errorf("ack packet = %x, want %x (type 0x02 + same sequence byte, no text)", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("client never acked the pushed server message — a real server would drop this connection")
	}
}

// ---------------------------------------------------------------------
// 6. Keepalive fires.
// ---------------------------------------------------------------------

func TestBattlEyeKeepaliveFires(t *testing.T) {
	pc := newBEFakeServer(t)
	host, port := beFakeAddr(t, pc)

	kaCh := make(chan []byte, 1)

	go func() {
		buf := make([]byte, 2048)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			pkt := append([]byte(nil), buf[:n]...)
			switch pkt[7] {
			case beTypeLogin:
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeLogin, []byte{beLoginOK}), addr)
			case beTypeCommand:
				payload := pkt[8:]
				if len(payload) == 1 {
					// No command text beyond the sequence byte: a keepalive.
					select {
					case kaCh <- pkt:
					default:
					}
					continue
				}
				seq := pkt[8]
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeCommand, append([]byte{seq}, []byte("ok")...)), addr)
			}
		}
	}()

	client := shortClient(host, port, "pw")
	client.keepaliveInterval = 20 * time.Millisecond
	defer func() { _ = client.Close() }()

	if _, err := client.Exec("hello"); err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	select {
	case pkt := <-kaCh:
		if pkt[7] != beTypeCommand {
			t.Errorf("keepalive packet type = 0x%02x, want 0x%02x", pkt[7], beTypeCommand)
		}
		if len(pkt) != 9 {
			t.Errorf("keepalive packet length = %d, want 9 (header + type + seq, no command text)", len(pkt))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no keepalive packet arrived within 2s of a 20ms keepaliveInterval")
	}
}

// ---------------------------------------------------------------------
// 7. Sequence wraps 0xFF -> 0x00.
// ---------------------------------------------------------------------

func TestBattlEyeSequenceWraps(t *testing.T) {
	pc := newBEFakeServer(t)
	host, port := beFakeAddr(t, pc)

	seqCh := make(chan byte, 8)

	go func() {
		buf := make([]byte, 2048)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			pkt := append([]byte(nil), buf[:n]...)
			switch pkt[7] {
			case beTypeLogin:
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeLogin, []byte{beLoginOK}), addr)
			case beTypeCommand:
				seq := pkt[8]
				seqCh <- seq
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeCommand, append([]byte{seq}, []byte("ok")...)), addr)
			}
		}
	}()

	client := shortClient(host, port, "pw")
	defer func() { _ = client.Close() }()

	// First Exec establishes the connection; per the spec, sequence
	// numbers start at 0.
	if _, err := client.Exec("first"); err != nil {
		t.Fatalf("first Exec failed: %v", err)
	}
	if got := <-seqCh; got != 0 {
		t.Errorf("first command sequence = %d, want 0 (spec: starts at 0)", got)
	}

	// Force the counter to the wrap boundary and confirm the NEXT
	// allocated sequence is exactly 0xFF, then 0x00 after that.
	client.ioMu.Lock()
	client.seq = 0xFF
	client.ioMu.Unlock()

	if _, err := client.Exec("second"); err != nil {
		t.Fatalf("second Exec failed: %v", err)
	}
	if got := <-seqCh; got != 0xFF {
		t.Errorf("second command sequence = %d, want 0xFF", got)
	}

	if _, err := client.Exec("third"); err != nil {
		t.Fatalf("third Exec failed: %v", err)
	}
	if got := <-seqCh; got != 0x00 {
		t.Errorf("third command sequence = %d, want 0x00 (wrapped from 0xFF)", got)
	}
}

// ---------------------------------------------------------------------
// Error paths: malformed/short packets, resolve failure, timeouts,
// password-function failure, write failure + reconnect.
// ---------------------------------------------------------------------

func TestBattlEyeParsePacketErrors(t *testing.T) {
	goodLogin := testBuildPacket(t, beTypeLogin, []byte{beLoginOK})

	badTerminator := append([]byte(nil), goodLogin...)
	badTerminator[6] = 0x00 // corrupt the 0xFF terminator

	badCRC := append([]byte(nil), goodLogin...)
	badCRC[2] ^= 0xFF // flip a CRC byte without touching the payload it covers

	cases := []struct {
		name string
		pkt  []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"too short for header", []byte{'B', 'E', 0, 0, 0, 0, 0xFF}},
		{"bad magic", append([]byte{'X', 'E'}, goodLogin[2:]...)},
		{"missing terminator", badTerminator},
		{"bad crc", badCRC},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := parsePacket(tc.pkt); err == nil {
				t.Errorf("parsePacket(%x): expected an error, got nil", tc.pkt)
			}
		})
	}
}

func TestBattlEyeResolveFailure(t *testing.T) {
	// "1:2:3" is not a valid IPv6 literal (too few groups, no "::"), and
	// JoinHostPort brackets any host containing a colon — so this fails
	// net.ResolveUDPAddr's local literal parse without ever touching the
	// network or DNS, making the test fast and deterministic.
	client := shortClient("1:2:3", 4, "pw")
	client.dialTimeout = 100 * time.Millisecond
	defer func() { _ = client.Close() }()

	_, err := client.Exec("x")
	if err == nil {
		t.Fatal("expected a resolve error, got nil")
	}
	if errors.Is(err, ErrAuth) {
		t.Errorf("a resolve failure must not be classified as ErrAuth, got: %v", err)
	}
}

func TestBattlEyeLoginTimeout(t *testing.T) {
	pc := newBEFakeServer(t)
	host, port := beFakeAddr(t, pc)
	// Never reply to anything — simulates "RCon not enabled" per the spec
	// (a server with no RCon password configured sends no reply at all).

	client := shortClient(host, port, "pw")
	client.dialTimeout = 100 * time.Millisecond
	defer func() { _ = client.Close() }()

	start := time.Now()
	_, err := client.Exec("x")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a login timeout error, got nil")
	}
	if errors.Is(err, ErrAuth) {
		t.Errorf("a timeout (no reply at all) must not be classified as ErrAuth (that's for an explicit 0x00 rejection), got: %v", err)
	}
	if elapsed > time.Second {
		t.Errorf("login timeout took %v, should respect dialTimeout (100ms)", elapsed)
	}
}

func TestBattlEyeCommandTimeoutRetransmits(t *testing.T) {
	pc := newBEFakeServer(t)
	host, port := beFakeAddr(t, pc)

	var cmdPackets int
	cmdCh := make(chan struct{}, 8)

	go func() {
		buf := make([]byte, 2048)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			pkt := append([]byte(nil), buf[:n]...)
			switch pkt[7] {
			case beTypeLogin:
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeLogin, []byte{beLoginOK}), addr)
			case beTypeCommand:
				// Deliberately never reply, to force Exec through its
				// retransmit-once-then-give-up path.
				cmdCh <- struct{}{}
			}
		}
	}()

	client := shortClient(host, port, "pw")
	client.ackTimeout = 50 * time.Millisecond
	client.execDeadline = 200 * time.Millisecond
	defer func() { _ = client.Close() }()

	_, err := client.Exec("neverreplied")
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	if errors.Is(err, ErrAuth) {
		t.Errorf("a command timeout must not be classified as ErrAuth, got: %v", err)
	}

	drain := time.After(300 * time.Millisecond)
countLoop:
	for {
		select {
		case <-cmdCh:
			cmdPackets++
		case <-drain:
			break countLoop
		}
	}
	if cmdPackets != 2 {
		t.Errorf("server saw %d command packets, want 2 (original + exactly one retransmit)", cmdPackets)
	}
}

func TestBattlEyeCloseNeverConnected(t *testing.T) {
	client := NewBattlEye("127.0.0.1", 0, func() (string, error) { return "pw", nil })
	if err := client.Close(); err != nil {
		t.Errorf("Close() on a never-connected client returned %v, want nil", err)
	}
}

func TestBattlEyePasswordFuncError(t *testing.T) {
	client := NewBattlEye("127.0.0.1", 2305, func() (string, error) {
		return "", errors.New("secret store unavailable")
	})
	defer func() { _ = client.Close() }()

	_, err := client.Exec("x")
	if err == nil {
		t.Fatal("expected an error when the password function fails, got nil")
	}
	if !strings.Contains(err.Error(), "secret store unavailable") {
		t.Errorf("expected the underlying error to be wrapped in, got: %v", err)
	}
}

func TestBattlEyeWriteFailureTriggersReconnect(t *testing.T) {
	pc := newBEFakeServer(t)
	host, port := beFakeAddr(t, pc)

	go func() {
		buf := make([]byte, 2048)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			pkt := append([]byte(nil), buf[:n]...)
			switch pkt[7] {
			case beTypeLogin:
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeLogin, []byte{beLoginOK}), addr)
			case beTypeCommand:
				seq := pkt[8]
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeCommand, append([]byte{seq}, []byte("ok")...)), addr)
			}
		}
	}()

	client := shortClient(host, port, "pw")
	defer func() { _ = client.Close() }()

	if _, err := client.Exec("first"); err != nil {
		t.Fatalf("first Exec failed: %v", err)
	}

	// Simulate the underlying socket dying without going through Close()
	// — e.g. the OS reclaiming the fd. The struct's conn field stays
	// non-nil (only dropLocked clears it), so the next Exec's Write must
	// itself surface the failure and drop the connection, rather than
	// panicking or wedging.
	client.mu.Lock()
	_ = client.conn.Close()
	client.mu.Unlock()

	if _, err := client.Exec("second"); err == nil {
		t.Fatal("expected an error from Exec once the socket died underneath it")
	}

	// A later Exec must transparently redial and succeed.
	got, err := client.Exec("third")
	if err != nil {
		t.Fatalf("Exec should succeed after reconnect: %v", err)
	}
	if got != "ok" {
		t.Errorf("Exec = %q, want %q", got, "ok")
	}
}
