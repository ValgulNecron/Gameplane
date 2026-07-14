// battleye.go implements the BattlEye RCon protocol (v2) used by DayZ,
// Arma, and other BattlEye-protected servers.
//
// Protocol reference: https://www.battleye.com/downloads/BERConProtocol.txt
//
// Wire format (UDP — the game server's own UDP port, default RCon port
// 2305, configured separately via RConPort in beserver_x64.cfg):
//
//	'B'(0x42) | 'E'(0x45) | 4-byte CRC32 (little-endian) | 0xFF | type | payload...
//
// Per the spec, the CRC32 covers "the subsequent bytes" — everything from
// the 0xFF terminator onward (0xFF + type + payload), NOT the whole packet
// and NOT just the payload. Standard IEEE polynomial
// (hash/crc32.ChecksumIEEE, same table Go uses for crc32.IEEE).
//
// Packet types:
//
//   - 0x00 login: payload is the plaintext password (no sequence byte).
//     Reply: 0x00 + (0x01 accepted | 0x00 rejected). No reply at all means
//     RCon isn't enabled on the server.
//
//   - 0x01 command: payload is a 1-byte sequence number (starts at 0,
//     wraps 0xFF -> 0x00) then the ASCII command. The reply echoes the
//     type and sequence, then either nothing, the response text, or — when
//     the response doesn't fit in one packet — a fragmentation header
//     (0x00 + total-packet-count + 0-based packet-index) followed by that
//     page's chunk of the response. Pages can arrive out of order and are
//     reassembled by index, never by arrival order.
//
//   - 0x02 server message: the server pushes async console lines (chat,
//     connects, kills) unprompted, each carrying its own sequence number.
//     The client MUST ack each one (0x02 + that sequence byte) — BE
//     retries an unacked message for ~10 seconds (5 attempts) and then
//     drops the client from its authenticated list.
//
// Keepalive: BE also drops a client that sends no command packets for more
// than 45 seconds. An empty command packet (0x01 + sequence, no command
// text) counts, so this client ticks one from a background goroutine
// roughly every 30s whenever no real command has been sent more recently.
//
// # Architecture
//
// UDP has no delivery guarantees and no built-in request/response
// correlation, so — unlike the TCP-based clients in this package — a
// single background goroutine owns all reads off the socket: it
// classifies every inbound packet by type, immediately acks server
// messages, reassembles (possibly out-of-order) fragmented command
// replies, and routes completed replies to whichever Exec call is
// waiting on that sequence number via a channel. Exec itself only writes
// the request and blocks on that channel with a deadline.
//
// Because a single dropped request or reply is indistinguishable from
// server silence, Exec retransmits the exact same command packet (same
// sequence number, so a merely-delayed original reply still satisfies
// the same waiter) once after a short ack window before giving up.
package rcon

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	beTypeLogin   byte = 0x00
	beTypeCommand byte = 0x01
	beTypeMessage byte = 0x02

	beLoginOK byte = 0x01

	defaultBattlEyePort = 2305

	// defaultBattlEyeDialTimeout bounds resolving the address and waiting
	// for the login reply. UDP "dialing" itself never blocks (there's no
	// handshake), so in practice this only bounds the login round trip.
	defaultBattlEyeDialTimeout = 5 * time.Second

	// defaultBattlEyeAckTimeout bounds the first wait for a command
	// reply before Exec retransmits once (see the package doc comment).
	defaultBattlEyeAckTimeout = 2 * time.Second

	// defaultBattlEyeExecDeadline bounds the WHOLE command exchange,
	// including the retransmit. Must be comfortably larger than
	// defaultBattlEyeAckTimeout or the post-retransmit wait collapses to
	// nothing.
	defaultBattlEyeExecDeadline = 10 * time.Second

	// defaultBattlEyeKeepalive ticks an empty command packet well inside
	// the spec's 45-second drop window.
	defaultBattlEyeKeepalive = 30 * time.Second

	// beReadBufSize bounds one UDP read. BE payloads are small ASCII
	// text, but a datagram can legally be up to ~65KB.
	beReadBufSize = 65535
)

// battlEyeReply is delivered to a waiting Exec call once its command's
// reply (single-packet or fully reassembled) has arrived, or once the
// connection has failed out from under it.
type battlEyeReply struct {
	body string
	err  error
}

// battlEyeLoginResult is delivered to ensureLocked once the login reply
// arrives, or once the connection fails before it does.
type battlEyeLoginResult struct {
	ok  bool
	err error
}

// battlEyeFragments accumulates a multi-packet command reply, keyed by
// page index so pages landing out of order still reassemble correctly.
type battlEyeFragments struct {
	total  int
	chunks map[int]string
}

// BattlEye is a lazy, auto-reconnecting client for the BattlEye RCon
// protocol (DayZ, Arma). It's safe for use from multiple goroutines;
// Exec calls are serialized on a single UDP socket via mu, mirroring the
// other clients in this package.
type BattlEye struct {
	addr   string
	passFn PassFn

	// dialTimeout/ackTimeout/execDeadline/keepaliveInterval are struct
	// fields (not package consts) so tests can shrink them — mirrors
	// telnet.go's connectTimeout/responseDeadline and websocket.go's
	// dialTimeout/execDeadline.
	dialTimeout       time.Duration
	ackTimeout        time.Duration
	execDeadline      time.Duration
	keepaliveInterval time.Duration

	// mu serializes Exec/ensureLocked/Close — the same "one operation at
	// a time" contract every client in this package makes. It is held
	// for the full duration of an Exec call, including the blocking wait
	// for a reply.
	mu   sync.Mutex
	conn *net.UDPConn

	// ioMu guards seq/waiters/partial/loginCh — the state the background
	// readLoop and keepaliveLoop goroutines touch concurrently with a
	// caller that's blocked inside Exec/ensureLocked holding mu. It is
	// deliberately a SEPARATE lock from mu: mu is held across a blocking
	// channel receive, and if readLoop needed mu too to deliver into that
	// channel, the two would deadlock (the reader waiting on a lock the
	// writer won't release until the reader delivers).
	ioMu    sync.Mutex
	seq     byte
	waiters map[byte]chan battlEyeReply
	partial map[byte]*battlEyeFragments
	loginCh chan battlEyeLoginResult

	stopKeepalive chan struct{}
	keepaliveDone chan struct{}
	readDone      chan struct{}
}

// NewBattlEye builds a BattlEye RCon client. The UDP socket is opened and
// logged in lazily on the first Exec.
func NewBattlEye(host string, port int, pass PassFn) *BattlEye {
	if port == 0 {
		port = defaultBattlEyePort
	}
	return &BattlEye{
		addr:              net.JoinHostPort(host, fmt.Sprint(port)),
		passFn:            pass,
		dialTimeout:       defaultBattlEyeDialTimeout,
		ackTimeout:        defaultBattlEyeAckTimeout,
		execDeadline:      defaultBattlEyeExecDeadline,
		keepaliveInterval: defaultBattlEyeKeepalive,
	}
}

// Exec sends one command and returns the (possibly reassembled) reply.
func (c *BattlEye) Exec(cmd string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureLocked(); err != nil {
		return "", err
	}

	conn := c.conn
	seq := c.nextSeq()
	ch := make(chan battlEyeReply, 1)
	c.ioMu.Lock()
	c.waiters[seq] = ch
	c.ioMu.Unlock()
	defer func() {
		c.ioMu.Lock()
		delete(c.waiters, seq)
		delete(c.partial, seq)
		c.ioMu.Unlock()
	}()

	pkt := buildCommandPacket(seq, cmd)
	if _, err := conn.Write(pkt); err != nil {
		c.dropLocked()
		return "", fmt.Errorf("battleye rcon exec %q: %w", cmd, err)
	}

	ackTimeout := c.ackTimeout
	if ackTimeout <= 0 {
		ackTimeout = defaultBattlEyeAckTimeout
	}
	execDeadline := c.execDeadline
	if execDeadline <= 0 {
		execDeadline = defaultBattlEyeExecDeadline
	}

	select {
	case r := <-ch:
		if r.err != nil {
			return "", fmt.Errorf("battleye rcon exec %q: %w", cmd, r.err)
		}
		return r.body, nil
	case <-time.After(ackTimeout):
		// UDP gives no delivery guarantee in either direction, so a
		// dropped request and a dropped reply both look like silence
		// from here. Retransmit the identical packet (same sequence
		// number) once: if the original reply was merely delayed rather
		// than lost, it still satisfies this same waiter when it shows
		// up (handleCommandReply delivers at most once per seq per
		// Exec call — see the non-blocking send there). We do not keep
		// retrying beyond this single retransmit; a server that's still
		// silent after that is treated as unreachable.
		if _, err := conn.Write(pkt); err != nil {
			c.dropLocked()
			return "", fmt.Errorf("battleye rcon exec %q (retransmit): %w", cmd, err)
		}
		remaining := execDeadline - ackTimeout
		if remaining <= 0 {
			remaining = ackTimeout
		}
		select {
		case r := <-ch:
			if r.err != nil {
				return "", fmt.Errorf("battleye rcon exec %q: %w", cmd, r.err)
			}
			return r.body, nil
		case <-time.After(remaining):
			// A command that never gets a reply even after a retransmit
			// suggests this UDP session is wedged (or the server died),
			// not just one lost packet. Drop the connection so the next
			// Exec re-logs-in from a clean state rather than piling more
			// unanswered commands onto a socket that may never recover.
			c.dropLocked()
			return "", fmt.Errorf("battleye rcon exec %q: timed out waiting for reply", cmd)
		}
	}
}

// Close shuts down the underlying connection and background goroutines.
func (c *BattlEye) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.dropLocked()
}

func (c *BattlEye) ensureLocked() error {
	if c.conn != nil {
		return nil
	}

	pw, err := c.passFn()
	if err != nil {
		return fmt.Errorf("battleye rcon: resolve password: %w", err)
	}

	raddr, err := net.ResolveUDPAddr("udp", c.addr)
	if err != nil {
		return fmt.Errorf("battleye rcon: resolve %s: %w", c.addr, err)
	}
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		return fmt.Errorf("battleye rcon: dial %s: %w", c.addr, err)
	}

	// Reset all per-connection state before anything can observe it
	// concurrently — readLoop/keepaliveLoop aren't started yet, so no
	// locking is needed for this part.
	c.conn = conn
	c.seq = 0
	c.waiters = make(map[byte]chan battlEyeReply)
	c.partial = make(map[byte]*battlEyeFragments)
	loginCh := make(chan battlEyeLoginResult, 1)
	c.loginCh = loginCh

	readDone := make(chan struct{})
	c.readDone = readDone
	go c.readLoop(conn, readDone)

	if _, err := conn.Write(buildPacket(beTypeLogin, []byte(pw))); err != nil {
		c.dropLocked()
		return fmt.Errorf("battleye rcon: send login: %w", err)
	}

	dialTimeout := c.dialTimeout
	if dialTimeout <= 0 {
		dialTimeout = defaultBattlEyeDialTimeout
	}
	select {
	case res := <-loginCh:
		if res.err != nil {
			c.dropLocked()
			return fmt.Errorf("battleye rcon: login: %w", res.err)
		}
		if !res.ok {
			c.dropLocked()
			return fmt.Errorf("battleye rcon: %w", ErrAuth)
		}
	case <-time.After(dialTimeout):
		c.dropLocked()
		return fmt.Errorf("battleye rcon: login timed out after %s (no reply — RCon may not be enabled on the server)", dialTimeout)
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	c.stopKeepalive = stop
	c.keepaliveDone = done
	go c.keepaliveLoop(conn, stop, done)

	return nil
}

// dropLocked tears down the connection and background goroutines, if
// any are running. Safe to call when nothing is connected. Must be
// called with mu held.
func (c *BattlEye) dropLocked() error {
	if c.conn == nil {
		return nil
	}

	if c.stopKeepalive != nil {
		close(c.stopKeepalive)
		<-c.keepaliveDone
		c.stopKeepalive = nil
		c.keepaliveDone = nil
	}

	err := c.conn.Close()
	if c.readDone != nil {
		// conn.Close unblocks readLoop's pending Read; wait for it to
		// notice, fail out any waiters, and exit before we return, so a
		// caller that immediately calls ensureLocked again never races
		// the previous readLoop's cleanup.
		<-c.readDone
		c.readDone = nil
	}
	c.conn = nil
	return err
}

// nextSeq allocates the next 1-byte sequence number, wrapping 0xFF back
// to 0x00 (plain byte overflow). Called by Exec (holding mu) and by
// keepaliveLoop (holding neither mu nor, at call time, ioMu — hence the
// lock here).
func (c *BattlEye) nextSeq() byte {
	c.ioMu.Lock()
	s := c.seq
	c.seq++
	c.ioMu.Unlock()
	return s
}

// readLoop owns every read off conn for the lifetime of one connection.
// It exits (and closes done) the moment Read fails, which happens when
// dropLocked closes conn.
func (c *BattlEye) readLoop(conn *net.UDPConn, done chan struct{}) {
	defer close(done)
	buf := make([]byte, beReadBufSize)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			c.failAll(err)
			return
		}
		pkt := make([]byte, n)
		copy(pkt, buf[:n])
		c.handlePacket(conn, pkt)
	}
}

// handlePacket classifies one inbound packet and routes it. Malformed
// packets (bad magic, bad CRC, truncated) are dropped silently: BE
// speaks UDP with no authentication on the wire other than the login
// password, so tolerating garbage is the correct response to both
// corruption and to unrelated traffic that might land on this socket.
func (c *BattlEye) handlePacket(conn *net.UDPConn, pkt []byte) {
	kind, payload, err := parsePacket(pkt)
	if err != nil {
		return
	}

	switch kind {
	case beTypeLogin:
		c.ioMu.Lock()
		ch := c.loginCh
		c.loginCh = nil
		c.ioMu.Unlock()
		if ch == nil {
			return
		}
		ok := len(payload) > 0 && payload[0] == beLoginOK
		select {
		case ch <- battlEyeLoginResult{ok: ok}:
		default:
		}

	case beTypeMessage:
		if len(payload) == 0 {
			return
		}
		// Ack immediately, per the package doc comment: BE retries an
		// unacked server message for ~10s and then drops us.
		seq := payload[0]
		_, _ = conn.Write(buildPacket(beTypeMessage, []byte{seq}))

	case beTypeCommand:
		c.handleCommandReply(payload)
	}
}

// handleCommandReply parses a command reply's payload — seq, then either
// nothing/plain text, or a fragmentation header (0x00 + total + index)
// followed by one page of a multi-packet response — and, once the full
// response is available, delivers it to the waiting Exec call (if any;
// keepalive replies have no waiter and are silently discarded).
func (c *BattlEye) handleCommandReply(payload []byte) {
	if len(payload) == 0 {
		return
	}
	seq := payload[0]
	rest := payload[1:]

	// The fragmentation header only exists on multi-packet responses; a
	// single-packet reply's body is plain ASCII text, which never starts
	// with the 0x00 the header requires as its first byte.
	if len(rest) >= 3 && rest[0] == 0x00 {
		total := int(rest[1])
		idx := int(rest[2])
		chunk := string(rest[3:])

		c.ioMu.Lock()
		frag := c.partial[seq]
		if frag == nil {
			frag = &battlEyeFragments{total: total, chunks: make(map[int]string)}
			c.partial[seq] = frag
		}
		frag.chunks[idx] = chunk

		complete := frag.total > 0 && len(frag.chunks) >= frag.total
		if complete {
			for i := 0; i < frag.total; i++ {
				if _, ok := frag.chunks[i]; !ok {
					complete = false
					break
				}
			}
		}
		var full string
		if complete {
			var b strings.Builder
			for i := 0; i < frag.total; i++ {
				b.WriteString(frag.chunks[i])
			}
			full = b.String()
			delete(c.partial, seq)
		}
		ch := c.waiters[seq]
		c.ioMu.Unlock()

		if !complete {
			return
		}
		if ch != nil {
			select {
			case ch <- battlEyeReply{body: full}:
			default:
			}
		}
		return
	}

	body := string(rest)
	c.ioMu.Lock()
	ch := c.waiters[seq]
	c.ioMu.Unlock()
	if ch != nil {
		select {
		case ch <- battlEyeReply{body: body}:
		default:
		}
	}
}

// failAll delivers err to every in-flight waiter (login and command) so
// a dead connection surfaces immediately instead of making callers wait
// out their full deadlines. Called from readLoop when Read fails.
func (c *BattlEye) failAll(err error) {
	c.ioMu.Lock()
	defer c.ioMu.Unlock()

	if ch := c.loginCh; ch != nil {
		select {
		case ch <- battlEyeLoginResult{err: err}:
		default:
		}
		c.loginCh = nil
	}
	for seq, ch := range c.waiters {
		select {
		case ch <- battlEyeReply{err: err}:
		default:
		}
		delete(c.waiters, seq)
	}
	c.partial = make(map[byte]*battlEyeFragments)
}

// keepaliveLoop sends an empty command packet on a fixed interval so BE
// doesn't drop this client for exceeding the spec's 45-second silence
// window. It shares the connection's sequence counter with Exec (both go
// through nextSeq) so a keepalive can never collide with an in-flight
// command's sequence number. Any reply the server sends to a keepalive
// has no registered waiter and is discarded by handleCommandReply.
func (c *BattlEye) keepaliveLoop(conn *net.UDPConn, stop <-chan struct{}, done chan struct{}) {
	defer close(done)

	interval := c.keepaliveInterval
	if interval <= 0 {
		interval = defaultBattlEyeKeepalive
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			seq := c.nextSeq()
			_, _ = conn.Write(buildCommandPacket(seq, ""))
		}
	}
}

// buildCommandPacket builds a type-0x01 packet: sequence byte followed
// by the (possibly empty, for a keepalive) ASCII command.
func buildCommandPacket(seq byte, cmd string) []byte {
	body := make([]byte, 0, 1+len(cmd))
	body = append(body, seq)
	body = append(body, cmd...)
	return buildPacket(beTypeCommand, body)
}

// buildPacket assembles one full BE packet: 'B' 'E' + little-endian
// CRC32 + 0xFF + type + payload. The CRC is computed over "the
// subsequent bytes" per the spec — 0xFF + type + payload — NOT the whole
// packet and NOT just payload. See the package doc comment; this byte
// range is pinned by TestBattlEyeCRCByteRange against a hand-computed,
// independently-derived expected value, not by round-tripping this
// function.
func buildPacket(kind byte, payload []byte) []byte {
	inner := make([]byte, 0, 2+len(payload))
	inner = append(inner, 0xFF, kind)
	inner = append(inner, payload...)

	crc := crc32.ChecksumIEEE(inner)

	pkt := make([]byte, 0, 6+len(inner))
	pkt = append(pkt, 'B', 'E')
	var crcBuf [4]byte
	binary.LittleEndian.PutUint32(crcBuf[:], crc)
	pkt = append(pkt, crcBuf[:]...)
	pkt = append(pkt, inner...)
	return pkt
}

// parsePacket validates and unwraps one inbound BE packet, returning its
// type and payload. Errors cover every way a packet can be malformed:
// too short to contain a header and type byte, wrong magic, a missing
// 0xFF terminator, or a CRC that doesn't match the bytes it's supposed
// to cover.
func parsePacket(pkt []byte) (kind byte, payload []byte, err error) {
	// 'B' 'E' + 4-byte CRC + 0xFF + type = 8 bytes minimum.
	const minLen = 8
	if len(pkt) < minLen {
		return 0, nil, fmt.Errorf("battleye rcon: short packet (%d bytes, need at least %d)", len(pkt), minLen)
	}
	if pkt[0] != 'B' || pkt[1] != 'E' {
		return 0, nil, fmt.Errorf("battleye rcon: bad magic %q", pkt[:2])
	}

	gotCRC := binary.LittleEndian.Uint32(pkt[2:6])
	inner := pkt[6:] // 0xFF + type + payload — the CRC's byte range per spec.

	if inner[0] != 0xFF {
		return 0, nil, fmt.Errorf("battleye rcon: missing 0xFF terminator (got 0x%02x)", inner[0])
	}
	if wantCRC := crc32.ChecksumIEEE(inner); gotCRC != wantCRC {
		return 0, nil, fmt.Errorf("battleye rcon: crc32 mismatch (got %08x, want %08x)", gotCRC, wantCRC)
	}

	kind = inner[1]
	payload = inner[2:]
	return kind, payload, nil
}
