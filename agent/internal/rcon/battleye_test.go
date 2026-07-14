package rcon

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash/crc32"
	"net"
	"strings"
	"sync/atomic"
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

// ---------------------------------------------------------------------
// 8. Transport death WHILE an Exec is blocked waiting for a reply —
// regression guard for DEFECT 1. TestBattlEyeWriteFailureTriggersReconnect
// above closes the socket with no Exec in flight, so the waiter map is
// empty and it's the subsequent Write that fails, never failAll. These two
// tests instead close the socket while a waiter IS registered, forcing
// conn.Read to fail inside readLoop and failAll to fan the error out to a
// blocked Exec — the only way to reach either "r.err != nil" branch in
// Exec. Both tests must FAIL against the pre-fix code, which left c.conn
// set after such a failure: the immediately following Exec would then
// write successfully into a dead socket with no reader and stall for the
// full execDeadline before finally reconnecting.
// ---------------------------------------------------------------------

// TestBattlEyeReadFailureDropsConnDuringExec kills the client's socket
// right after the fake server receives the (never-to-be-answered) command
// packet, so the failure lands on Exec's FIRST select (before ackTimeout
// fires) — the "case r := <-ch" branch around line ~239.
//
// A "warmup" Exec establishes the connection first and returns (releasing
// c.mu) before the blocked "stuck" Exec starts: c.mu is held for an
// Exec's ENTIRE duration, including while blocked waiting for a reply, so
// this test cannot lock client.mu to read client.conn while "stuck" is
// in flight — it must capture the conn reference while mu is free
// (right after the warmup call returns) and close that captured
// reference directly. net.Conn methods are safe for concurrent use, so
// closing it from the test goroutine while readLoop is blocked in Read
// and Exec has already completed its Write is fine.
func TestBattlEyeReadFailureDropsConnDuringExec(t *testing.T) {
	pc := newBEFakeServer(t)
	host, port := beFakeAddr(t, pc)

	cmdReceived := make(chan struct{}, 1)

	go func() {
		buf := make([]byte, 2048)
		cmdSeen := 0
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
				cmdSeen++
				if cmdSeen == 2 {
					// The "stuck" command: deliberately never reply — the
					// test's own socket-close below is what ends this
					// Exec, not a server reply.
					select {
					case cmdReceived <- struct{}{}:
					default:
					}
					continue
				}
				// The warmup command (1st) and the post-reconnect Exec's
				// command (3rd+) both get real replies.
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeCommand, append([]byte{seq}, []byte("ok")...)), addr)
			}
		}
	}()

	client := shortClient(host, port, "pw")
	client.ackTimeout = 3 * time.Second    // must not fire before the read failure does
	client.execDeadline = 10 * time.Second // the assertion below must be well under this
	defer func() { _ = client.Close() }()

	if _, err := client.Exec("warmup"); err != nil {
		t.Fatalf("warmup Exec failed: %v", err)
	}

	// mu is free now (the warmup Exec returned) — safe to read c.conn.
	client.mu.Lock()
	conn := client.conn
	client.mu.Unlock()
	if conn == nil {
		t.Fatal("client has no connection after warmup")
	}

	execDone := make(chan struct{})
	var execErr error
	start := time.Now()
	go func() {
		defer close(execDone)
		_, execErr = client.Exec("stuck")
	}()

	select {
	case <-cmdReceived:
	case <-time.After(2 * time.Second):
		t.Fatal("server never received the stuck command packet")
	}

	// Kill the client's own (already-captured) socket out from under the
	// blocked Exec, simulating the transport dying (e.g. an ICMP
	// port-unreachable) while a reply is awaited. This makes conn.Read
	// fail inside readLoop and triggers failAll — the code path DEFECT 1
	// lives in.
	_ = conn.Close()

	select {
	case <-execDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Exec did not return promptly after the read failure — DEFECT 1 not fixed")
	}
	elapsed := time.Since(start)

	if execErr == nil {
		t.Fatal("expected an error from the blocked Exec once its connection died")
	}
	if elapsed > time.Second {
		t.Errorf("blocked Exec took %v to return after the read failure, want well under execDeadline (%v)", elapsed, client.execDeadline)
	}

	// The IMMEDIATELY FOLLOWING Exec must succeed against the still-live
	// fake server. Before the fix, c.conn stayed set with no reader
	// behind it, so this Exec would write into the dead socket, get no
	// reply, and stall for the full execDeadline (retransmitting
	// pointlessly) instead of redialing.
	nextStart := time.Now()
	got, err := client.Exec("next")
	nextElapsed := time.Since(nextStart)
	if err != nil {
		t.Fatalf("Exec immediately after the read failure should succeed via reconnect, got: %v", err)
	}
	if got != "ok" {
		t.Errorf("Exec = %q, want %q", got, "ok")
	}
	if nextElapsed > time.Second {
		t.Errorf("the following Exec took %v — the dead connection was not dropped and reused instead of redialed", nextElapsed)
	}
}

// TestBattlEyeReadFailureDuringRetransmitWait is the same regression
// guard, but lets Exec's ackTimeout actually fire and retransmit first,
// so the read failure instead lands on the SECOND select (the
// post-retransmit wait) — the other "case r := <-ch" branch around line
// ~271. See TestBattlEyeReadFailureDropsConnDuringExec's doc comment for
// why this uses a warmup Exec to capture the conn reference instead of
// locking client.mu while "stuck" is in flight.
func TestBattlEyeReadFailureDuringRetransmitWait(t *testing.T) {
	pc := newBEFakeServer(t)
	host, port := beFakeAddr(t, pc)

	cmdCount := make(chan struct{}, 8)

	go func() {
		buf := make([]byte, 2048)
		cmdSeen := 0
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
				cmdSeen++
				if cmdSeen == 2 || cmdSeen == 3 {
					// cmd #2 is "stuck"'s original send, #3 is its
					// retransmit — never reply to either. The test's
					// socket-close is what ends this Exec.
					select {
					case cmdCount <- struct{}{}:
					default:
					}
					continue
				}
				// The warmup command (1st) and the post-reconnect Exec's
				// command (4th+) both get real replies.
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeCommand, append([]byte{seq}, []byte("ok")...)), addr)
			}
		}
	}()

	client := shortClient(host, port, "pw")
	client.ackTimeout = 50 * time.Millisecond // fire quickly so the retransmit happens fast
	client.execDeadline = 10 * time.Second    // the assertion below must be well under this
	defer func() { _ = client.Close() }()

	if _, err := client.Exec("warmup"); err != nil {
		t.Fatalf("warmup Exec failed: %v", err)
	}

	// mu is free now (the warmup Exec returned) — safe to read c.conn.
	client.mu.Lock()
	conn := client.conn
	client.mu.Unlock()
	if conn == nil {
		t.Fatal("client has no connection after warmup")
	}

	execDone := make(chan struct{})
	var execErr error
	start := time.Now()
	go func() {
		defer close(execDone)
		_, execErr = client.Exec("stuck")
	}()

	// Wait for BOTH the original "stuck" command and its retransmit,
	// proving we're now inside Exec's post-retransmit wait (the second
	// select).
	for i := 0; i < 2; i++ {
		select {
		case <-cmdCount:
		case <-time.After(2 * time.Second):
			t.Fatalf("server did not see %d command packets (retransmit) in time", i+1)
		}
	}

	// Kill the client's own (already-captured) socket out from under the
	// blocked Exec.
	_ = conn.Close()

	select {
	case <-execDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Exec did not return promptly after the read failure during the retransmit wait — DEFECT 1 not fixed")
	}
	elapsed := time.Since(start)

	if execErr == nil {
		t.Fatal("expected an error from the blocked Exec once its connection died")
	}
	if elapsed > 2*time.Second {
		t.Errorf("blocked Exec took %v to return after the read failure, want well under execDeadline (%v)", elapsed, client.execDeadline)
	}

	nextStart := time.Now()
	got, err := client.Exec("next")
	nextElapsed := time.Since(nextStart)
	if err != nil {
		t.Fatalf("Exec immediately after the read failure should succeed via reconnect, got: %v", err)
	}
	if got != "ok" {
		t.Errorf("Exec = %q, want %q", got, "ok")
	}
	if nextElapsed > time.Second {
		t.Errorf("the following Exec took %v — the dead connection was not dropped and reused instead of redialed", nextElapsed)
	}
}

// TestBattlEyeEnsureLockedRedialsAfterIdleDeath is the OTHER half of the
// DEFECT 1 regression guard: the connection dying while IDLE, with no
// Exec in flight, so failAll finds no waiters and nothing in Exec ever
// runs dropLocked for it. Before the fix, ensureLocked only checked "is
// c.conn non-nil", so it happily handed the next Exec a dead, reader-less
// socket, which would write into it, get no reply, and stall for a full
// execDeadline. This test kills the connection with nothing in flight,
// waits for readLoop to actually exit (proving the idle scenario, not a
// race with a write — that race is already covered by
// TestBattlEyeWriteFailureTriggersReconnect), and asserts the next Exec
// redials (a second login is observed) and returns promptly.
func TestBattlEyeEnsureLockedRedialsAfterIdleDeath(t *testing.T) {
	pc := newBEFakeServer(t)
	host, port := beFakeAddr(t, pc)

	loginCh := make(chan struct{}, 8)

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
				select {
				case loginCh <- struct{}{}:
				default:
				}
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeLogin, []byte{beLoginOK}), addr)
			case beTypeCommand:
				seq := pkt[8]
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeCommand, append([]byte{seq}, []byte("ok")...)), addr)
			}
		}
	}()

	client := shortClient(host, port, "pw")
	client.execDeadline = 2 * time.Second
	defer func() { _ = client.Close() }()

	if _, err := client.Exec("first"); err != nil {
		t.Fatalf("first Exec failed: %v", err)
	}
	select {
	case <-loginCh:
	case <-time.After(time.Second):
		t.Fatal("server never saw the first login")
	}

	// Kill the connection with NOTHING in flight, simulating the
	// connection dying while idle (e.g. a keepalive triggering an ICMP
	// port-unreachable).
	client.mu.Lock()
	conn := client.conn
	readDone := client.readDone
	client.mu.Unlock()
	if conn == nil || readDone == nil {
		t.Fatal("client has no live connection to kill")
	}
	_ = conn.Close()

	// Wait for readLoop to actually notice and exit, so this test
	// deterministically exercises the idle-death branch rather than
	// racing a write failure.
	select {
	case <-readDone:
	case <-time.After(2 * time.Second):
		t.Fatal("readLoop never exited after its connection was closed")
	}

	start := time.Now()
	got, err := client.Exec("second")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Exec after an idle connection death should redial and succeed, got: %v", err)
	}
	if got != "ok" {
		t.Errorf("Exec = %q, want %q", got, "ok")
	}
	if elapsed > time.Second {
		t.Errorf("Exec took %v to redial after an idle connection death, want well under execDeadline (%v)", elapsed, client.execDeadline)
	}

	select {
	case <-loginCh:
	case <-time.After(time.Second):
		t.Fatal("client never redialed (no second login observed) — DEFECT 1's idle-death path not fixed")
	}
}

// ---------------------------------------------------------------------
// 9. Auth-failure cooldown — DEFECT 2 regression guard, mirroring
// websocket.go's TestWebSocketAuthFailureCooldown.
// ---------------------------------------------------------------------

func TestBattlEyeAuthFailureCooldown(t *testing.T) {
	pc := newBEFakeServer(t)
	host, port := beFakeAddr(t, pc)

	var loginAttempts int32

	go func() {
		buf := make([]byte, 2048)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			pkt := append([]byte(nil), buf[:n]...)
			if pkt[7] == beTypeLogin {
				atomic.AddInt32(&loginAttempts, 1)
				// Always reject — a template configured with a wrong
				// RCon password.
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeLogin, []byte{0x00}), addr)
			}
		}
	}()

	client := shortClient(host, port, "wrong-password")
	client.authFailureCooldown = 100 * time.Millisecond
	defer func() { _ = client.Close() }()

	_, err1 := client.Exec("cmd1")
	if !errors.Is(err1, ErrAuth) {
		t.Fatalf("first Exec expected ErrAuth, got %v", err1)
	}
	if got := atomic.LoadInt32(&loginAttempts); got != 1 {
		t.Fatalf("expected 1 login attempt after the first Exec, got %d", got)
	}

	// Within the cooldown window: must NOT re-dial with the same
	// known-bad password.
	_, err2 := client.Exec("cmd2")
	if !errors.Is(err2, ErrAuth) {
		t.Fatalf("second Exec (within cooldown) expected ErrAuth, got %v", err2)
	}
	if got := atomic.LoadInt32(&loginAttempts); got != 1 {
		t.Fatalf("expected still 1 login attempt during the cooldown (no re-dial), got %d", got)
	}

	time.Sleep(150 * time.Millisecond)

	// After the cooldown expires: must attempt to re-dial (and fail
	// again, since the password is still wrong).
	_, err3 := client.Exec("cmd3")
	if !errors.Is(err3, ErrAuth) {
		t.Fatalf("third Exec (after cooldown) expected ErrAuth, got %v", err3)
	}
	if got := atomic.LoadInt32(&loginAttempts); got != 2 {
		t.Fatalf("expected 2 login attempts after the cooldown expired, got %d", got)
	}
}

// ---------------------------------------------------------------------
// 10. Fragment reassembly across a retransmit — DEFECT 3 regression
// guard.
// ---------------------------------------------------------------------

// TestBattlEyeFragmentResetsOnRetransmitTotalMismatch reproduces the
// DEFECT 3 scenario from the adversarial review: a 3-page reply's page 2
// is badly delayed (not permanently lost) so Exec's ackTimeout fires on
// only 2-of-3 pages and it retransmits. The retransmit gets a genuinely
// different, shorter reply (2 pages — e.g. a player left between
// attempts) that reuses the SAME sequence number, and only afterward does
// the first attempt's stale, delayed page finally arrive. The stale page
// must never merge into the retransmit's already-complete reply.
func TestBattlEyeFragmentResetsOnRetransmitTotalMismatch(t *testing.T) {
	pc := newBEFakeServer(t)
	host, port := beFakeAddr(t, pc)

	go func() {
		buf := make([]byte, 2048)
		cmdSeen := 0
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
				cmdSeen++
				switch cmdSeen {
				case 1:
					// Attempt 1: a 3-page reply. Pages 0 and 1 arrive
					// now; page 2 is badly delayed — sent below, AFTER
					// the retransmit's full reply, instead of lost.
					_, _ = pc.WriteTo(testBuildPacket(t, beTypeCommand, append([]byte{seq, 0x00, 3, 0}, []byte("Hello, ")...)), addr)
					_, _ = pc.WriteTo(testBuildPacket(t, beTypeCommand, append([]byte{seq, 0x00, 3, 1}, []byte("stale-World")...)), addr)
				case 2:
					// The retransmit (same seq): a genuinely different,
					// shorter 2-page reply.
					_, _ = pc.WriteTo(testBuildPacket(t, beTypeCommand, append([]byte{seq, 0x00, 2, 0}, []byte("Bye, ")...)), addr)
					_, _ = pc.WriteTo(testBuildPacket(t, beTypeCommand, append([]byte{seq, 0x00, 2, 1}, []byte("World")...)), addr)
					// Now deliver attempt 1's badly-delayed page 2.
					_, _ = pc.WriteTo(testBuildPacket(t, beTypeCommand, append([]byte{seq, 0x00, 3, 2}, []byte("!!!")...)), addr)
				}
			}
		}
	}()

	client := shortClient(host, port, "pw")
	client.ackTimeout = 80 * time.Millisecond
	client.execDeadline = 2 * time.Second
	defer func() { _ = client.Close() }()

	got, err := client.Exec("players")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if got != "Bye, World" {
		t.Errorf("Exec(players) = %q, want %q (the retransmit's own 2-page reply, with no stale page from the first attempt spliced on)", got, "Bye, World")
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

	// This Exec may go either way, and both outcomes are correct: it races the
	// background reader. If Exec gets there first it writes to the dead socket
	// and surfaces the failure; if the reader noticed the closed fd first it has
	// already signalled readDone, so ensureLocked drops the corpse and redials
	// transparently. Asserting either specific outcome would be asserting the
	// winner of a race, which is how you write a flaky test. What must hold is
	// that it returns promptly rather than wedging, and never reports the dead
	// socket as a rejected password.
	done := make(chan error, 1)
	go func() { _, err := client.Exec("second"); done <- err }()
	select {
	case err := <-done:
		if err != nil && errors.Is(err, ErrAuth) {
			t.Errorf("a dead socket must not be reported as an auth failure: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Exec wedged after the socket died underneath it")
	}

	// Whatever happened above, the client must be healthy again: the socket is
	// gone, so this can only pass by redialing.
	got, err := client.Exec("third")
	if err != nil {
		t.Fatalf("Exec should succeed after reconnect: %v", err)
	}
	if got != "ok" {
		t.Errorf("Exec = %q, want %q", got, "ok")
	}
}
