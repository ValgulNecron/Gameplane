package rcon

import (
	"errors"
	"io"
	"net"
	"syscall"
	"testing"
)

// timeoutErr is a minimal net.Error that reports Timeout() == true, used to
// exercise isTimeout's positive branch without depending on a real network
// deadline firing.
type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

// nonTimeoutNetErr is a net.Error that is NOT a timeout (e.g. a connection
// reset surfaced through the net.Error interface).
type nonTimeoutNetErr struct{}

func (nonTimeoutNetErr) Error() string   { return "connection reset" }
func (nonTimeoutNetErr) Timeout() bool   { return false }
func (nonTimeoutNetErr) Temporary() bool { return false }

func TestIsTimeout(t *testing.T) {
	if !isTimeout(timeoutErr{}) {
		t.Error("a net.Error with Timeout()==true should be reported as a timeout")
	}
	if isTimeout(nonTimeoutNetErr{}) {
		t.Error("a net.Error with Timeout()==false should not be reported as a timeout")
	}
	if isTimeout(io.EOF) {
		t.Error("a plain non-net.Error like io.EOF should not be reported as a timeout")
	}
	if isTimeout(nil) {
		t.Error("nil should not be reported as a timeout")
	}
	// Wrapped timeout errors must still be detected via errors.As.
	wrapped := &net.OpError{Op: "read", Err: timeoutErr{}}
	if !isTimeout(wrapped) {
		t.Error("a wrapped net.Error timeout should still be detected via errors.As")
	}
}

func TestSplitPalworldCommand(t *testing.T) {
	cases := []struct {
		in, first, rest string
	}{
		{"", "", ""},
		{"   ", "", ""},
		{"Shutdown", "Shutdown", ""},
		{"  Shutdown  ", "Shutdown", ""},
		{"Shutdown 30 Server is restarting", "Shutdown", "30 Server is restarting"},
		{"Broadcast   hello   world  ", "Broadcast", "hello   world"},
	}
	for _, c := range cases {
		first, rest := splitPalworldCommand(c.in)
		if first != c.first || rest != c.rest {
			t.Errorf("splitPalworldCommand(%q) = (%q, %q), want (%q, %q)", c.in, first, rest, c.first, c.rest)
		}
	}
}

func TestIsAuthCloseSignal(t *testing.T) {
	if isAuthCloseSignal(nil) {
		t.Error("nil should not be an auth close signal")
	}
	if !isAuthCloseSignal(io.EOF) {
		t.Error("io.EOF should be treated as an auth close signal")
	}
	if isAuthCloseSignal(errors.New("some unrelated error")) {
		t.Error("an unrelated plain error should not be an auth close signal")
	}

	// A peer reset surfaced as a read *net.OpError wrapping ECONNRESET.
	reset := &net.OpError{Op: "read", Err: syscall.ECONNRESET}
	if !isAuthCloseSignal(reset) {
		t.Error("a read ECONNRESET should be treated as an auth close signal")
	}

	// The same errno on a non-"read" op must NOT be treated as an auth
	// signal (only a read-side reset is disambiguated as auth).
	writeReset := &net.OpError{Op: "write", Err: syscall.ECONNRESET}
	if isAuthCloseSignal(writeReset) {
		t.Error("a write-side ECONNRESET should not be treated as an auth close signal")
	}

	// A read-side timeout is neither a close frame, EOF, nor ECONNRESET.
	timeoutOp := &net.OpError{Op: "read", Err: timeoutErr{}}
	if isAuthCloseSignal(timeoutOp) {
		t.Error("a read timeout should not be treated as an auth close signal")
	}
}
