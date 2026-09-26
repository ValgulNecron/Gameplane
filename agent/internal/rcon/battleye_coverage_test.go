package rcon

import (
	"net"
	"testing"
	"time"
)

// --- Direct unit calls on handlePacket/handleCommandReply/deliverFragment,
// no socket, no timing. -----------------------------------------------------

// TestBattlEyeHandlePacketMalformedDropped covers the parsePacket-error
// branch of handlePacket: a packet too short to even contain a header must
// be silently dropped, not routed anywhere.
func TestBattlEyeHandlePacketMalformedDropped(t *testing.T) {
	c := &BattlEye{
		waiters: make(map[byte]chan battlEyeReply),
		partial: make(map[byte]*battlEyeFragments),
	}
	loginCh := make(chan battlEyeLoginResult, 1)
	c.loginCh = loginCh

	c.handlePacket(nil, []byte("short")) // < 8 bytes: fails parsePacket's length check

	select {
	case res := <-loginCh:
		t.Fatalf("a malformed packet must not be routed anywhere, got login result %+v", res)
	default:
	}
	if c.loginCh != loginCh {
		t.Error("a malformed packet must not consume loginCh")
	}
}

// TestBattlEyeHandlePacketLoginNoWaiter covers the ch==nil guard in the
// login branch of handlePacket: a login reply arriving when nothing is
// waiting on it (e.g. a stray duplicate) must return cleanly rather than
// sending on a nil channel.
func TestBattlEyeHandlePacketLoginNoWaiter(t *testing.T) {
	c := &BattlEye{} // loginCh is nil
	pkt := testBuildPacket(t, beTypeLogin, []byte{beLoginOK})

	c.handlePacket(nil, pkt) // must return without panicking or blocking

	if c.loginCh != nil {
		t.Error("handlePacket must not set loginCh from a login reply")
	}
}

// TestBattlEyeHandlePacketEmptyMessagePayload covers the empty-payload
// guard in the beTypeMessage branch. conn is nil: without the guard
// returning first, the ack Write below it would panic on a nil
// *net.UDPConn instead of the payload being silently dropped.
func TestBattlEyeHandlePacketEmptyMessagePayload(t *testing.T) {
	c := &BattlEye{}
	pkt := testBuildPacket(t, beTypeMessage, nil)

	c.handlePacket(nil, pkt) // must return before touching the nil conn
}

// TestBattlEyeHandleCommandReplyEmptyPayload covers the empty-payload
// guard in handleCommandReply: an empty payload has no sequence byte, so
// without the guard, indexing payload[0] would panic.
func TestBattlEyeHandleCommandReplyEmptyPayload(t *testing.T) {
	c := &BattlEye{
		waiters: make(map[byte]chan battlEyeReply),
	}
	c.handleCommandReply(nil) // must return before indexing payload[0]

	if len(c.waiters) != 0 {
		t.Error("an empty command reply must not register or deliver anything")
	}
}

// TestBattlEyeDeliverFragmentNoWaiter covers deliverFragment's early
// return when no Exec call is waiting on this sequence number: the
// fragment must be dropped, not accumulated into c.partial (which nothing
// but a later failAll would ever clear).
func TestBattlEyeDeliverFragmentNoWaiter(t *testing.T) {
	c := &BattlEye{
		waiters: make(map[byte]chan battlEyeReply),
		partial: make(map[byte]*battlEyeFragments),
	}
	c.deliverFragment(5, 2, 0, "chunk")

	if _, ok := c.partial[5]; ok {
		t.Error("a fragment with no registered waiter must not be accumulated into c.partial")
	}
}

// --- Exec write failure, deterministic (no ack/exec timing race). ----------

// TestBattlEyeExecWriteFailsOnClosedSocket builds a client whose UDP
// socket is already closed and whose readLoop was never started
// (readDone == nil), so ensureLocked's readDone check takes the default
// branch and returns immediately without dialing. Exec's own conn.Write
// then fails deterministically against the closed socket.
func TestBattlEyeExecWriteFailsOnClosedSocket(t *testing.T) {
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1})
	if err != nil {
		t.Fatalf("dial udp: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("close udp conn: %v", err)
	}

	c := &BattlEye{
		conn:    conn,
		waiters: make(map[byte]chan battlEyeReply),
		partial: make(map[byte]*battlEyeFragments),
	}

	_, err = c.Exec("status")
	if err == nil {
		t.Fatal("Exec on an already-closed UDP socket should fail")
	}
	if c.conn != nil {
		t.Error("a failed write must drop the connection (c.conn should be nil)")
	}
}

// --- Zeroed timeouts fall back to defaults, against a replying fake
// server so the test never waits out the real (multi-second) defaults. ----

// TestBattlEyeZeroedTimeoutsFallBackToDefaults covers the <=0 fallback
// branches for ackTimeout, execDeadline and dialTimeout (in ensureLocked
// and Exec) plus keepaliveInterval (in keepaliveLoop, evaluated once at
// goroutine start), all in one successful Exec against a fake server that
// always answers immediately.
func TestBattlEyeZeroedTimeoutsFallBackToDefaults(t *testing.T) {
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

	client := NewBattlEye(host, port, func() (string, error) { return "pw", nil })
	client.dialTimeout = 0
	client.ackTimeout = 0
	client.execDeadline = 0
	client.keepaliveInterval = 0
	defer func() { _ = client.Close() }()

	got, err := client.Exec("status")
	if err != nil {
		t.Fatalf("Exec with zeroed timeouts should fall back to defaults and succeed: %v", err)
	}
	if got != "ok" {
		t.Errorf("Exec = %q, want %q", got, "ok")
	}
}

// TestBattlEyeExecDeadlineNotAboveAckTimeoutUsesAckTimeoutForRemaining
// covers the "remaining <= 0" fallback in Exec's retransmit wait: with
// execDeadline <= ackTimeout, the post-retransmit wait would be zero or
// negative, so Exec must fall back to waiting ackTimeout again instead of
// returning instantly. The fake server answers the login but never the
// command, so both waits are exhausted deterministically.
func TestBattlEyeExecDeadlineNotAboveAckTimeoutUsesAckTimeoutForRemaining(t *testing.T) {
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
			if pkt[7] == beTypeLogin {
				_, _ = pc.WriteTo(testBuildPacket(t, beTypeLogin, []byte{beLoginOK}), addr)
			}
			// Command packets are deliberately never answered.
		}
	}()

	client := NewBattlEye(host, port, func() (string, error) { return "pw", nil })
	client.dialTimeout = 500 * time.Millisecond
	client.ackTimeout = 40 * time.Millisecond
	client.execDeadline = 40 * time.Millisecond // <= ackTimeout
	defer func() { _ = client.Close() }()

	start := time.Now()
	_, err := client.Exec("status")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a timeout error from a server that never replies to the command")
	}
	// Two waits of ~ackTimeout each (initial + remaining-fallback), so this
	// must take noticeably longer than a single ackTimeout, and (as a
	// generous safety net, not a tight bound) well under 2s.
	if elapsed < client.ackTimeout {
		t.Errorf("Exec returned too fast (%v) for two %v waits; remaining fallback may not have applied", elapsed, client.ackTimeout)
	}
	if elapsed > 2*time.Second {
		t.Errorf("Exec took too long (%v)", elapsed)
	}
}
