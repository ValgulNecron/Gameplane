package capture

import (
	"strings"
	"testing"
)

// TestCompileFilter_ValidFilter tests that a valid BPF expression is accepted.
func TestCompileFilter_ValidFilter(t *testing.T) {
	expr := "tcp port 8080"
	f, err := CompileFilter(expr)
	if err != nil {
		t.Fatalf("CompileFilter(%q) failed: %v", expr, err)
	}
	if f == nil {
		t.Fatal("CompileFilter returned nil Filter with nil error")
	}
	if f.Expression() != expr {
		t.Errorf("Expression() = %q; want %q", f.Expression(), expr)
	}
	if len(f.Instructions()) == 0 {
		t.Error("Instructions() returned empty slice; expected a non-empty compiled BPF program")
	}
}

// TestCompileFilter_InvalidFilter tests that an invalid BPF expression is rejected
// before any capture starts, with a clear error.
func TestCompileFilter_InvalidFilter(t *testing.T) {
	// A malformed port primitive fails in the compiler itself, with no name
	// resolution involved, so this test asserts on syntax rather than on
	// whatever the ambient resolver happens to do with a bogus hostname.
	expr := "tcp port notaport"
	f, err := CompileFilter(expr)
	if err == nil {
		t.Fatalf("CompileFilter(%q) succeeded; want error", expr)
	}
	if f != nil {
		t.Error("CompileFilter returned non-nil Filter after error")
	}
	// Error message must include "invalid filter" per contract.
	if !strings.Contains(err.Error(), "invalid filter") {
		t.Errorf("error message = %q; want substring 'invalid filter'", err.Error())
	}
}

// TestCompileFilter_EmptyFilter tests that an empty filter is rejected.
func TestCompileFilter_EmptyFilter(t *testing.T) {
	f, err := CompileFilter("")
	if err == nil {
		t.Fatal("CompileFilter(\"\") succeeded; want error")
	}
	if f != nil {
		t.Error("CompileFilter returned non-nil Filter after error")
	}
}

// TestCompileFilter_VariousValidFilters tests a range of valid BPF expressions.
func TestCompileFilter_VariousValidFilters(t *testing.T) {
	tests := []string{
		"tcp port 80",
		"udp port 53",
		"ip src 192.168.1.1",
		"tcp and dst port 443",
		"udp port 25565 or tcp port 25565",
		"tcp port 22 or tcp port 23",
	}
	for _, expr := range tests {
		t.Run(expr, func(t *testing.T) {
			f, err := CompileFilter(expr)
			if err != nil {
				t.Fatalf("CompileFilter(%q) failed: %v", expr, err)
			}
			if f == nil {
				t.Fatal("CompileFilter returned nil Filter with nil error")
			}
			if f.Expression() != expr {
				t.Errorf("Expression() = %q; want %q", f.Expression(), expr)
			}
			if len(f.Instructions()) == 0 {
				t.Error("Instructions() returned empty slice")
			}
		})
	}
}

// TestFilter_NoPartialState ensures the rejection path leaves no partial state.
// This is verified implicitly: if CompileFilter returns an error, f is nil and
// the caller's code path that would store or use the Filter is not reached.
// A direct test would check that a failed compile leaves no file handles or
// goroutines open, but that's verified at the writer and HTTP handler level.
func TestFilter_NoPartialState(t *testing.T) {
	// Attempt to compile an invalid filter multiple times.
	// Each should cleanly fail with no state leakage between attempts.
	for i := 0; i < 3; i++ {
		f, err := CompileFilter("udp port notaport")
		if err == nil {
			t.Fatal("unexpected success")
		}
		if f != nil {
			t.Error("non-nil Filter after error")
		}
	}
}

// TestCompileFilter_RejectsUnimplementedProtocols pins down the reason bare
// "icmp" is refused: go-pcap parses it, but its compiler emits no test for it,
// so the resulting BPF program keeps every packet on the interface. Accepting
// such a filter would silently record traffic the user never asked for — game
// payloads, RCON, and the agent's own control-plane traffic — so it must be an
// error rather than a surprise.
func TestCompileFilter_RejectsUnimplementedProtocols(t *testing.T) {
	tests := []string{
		"icmp",
		"icmp or arp",
		"udp port 53 and icmp",
		"proto \\icmp",
		"ICMP",
		"esp",
		"vrrp",
	}

	for _, expr := range tests {
		t.Run(expr, func(t *testing.T) {
			f, err := CompileFilter(expr)
			if err == nil {
				t.Fatalf("CompileFilter(%q) succeeded; want a rejection", expr)
			}
			if f != nil {
				t.Error("CompileFilter returned non-nil Filter after error")
			}
			if !strings.Contains(err.Error(), "invalid filter") {
				t.Errorf("error message = %q; want substring 'invalid filter'", err.Error())
			}
		})
	}
}

// TestCompileFilter_CompiledProgramDiscriminates asserts that an accepted
// filter compiles to a program that actually tests the packet, which is the
// structural backstop behind the keyword list.
func TestCompileFilter_CompiledProgramDiscriminates(t *testing.T) {
	f, err := CompileFilter("udp port 25565")
	if err != nil {
		t.Fatalf("CompileFilter failed: %v", err)
	}
	if !testsPacketContents(f.Instructions()) {
		t.Error("compiled program has no conditional branch; it would match every packet")
	}
}
