package capture

import (
	"fmt"
	"strings"

	"github.com/packetcap/go-pcap/filter"
	"golang.org/x/net/bpf"
)

// Filter represents a compiled BPF filter for packet capture.
// It must be validated before capture starts to provide early rejection
// of invalid expressions (FR-003).
type Filter struct {
	instructions []bpf.Instruction // compiled BPF program, ready for bpf.Assemble
	expr         string            // original expression for logging/debugging
}

// unsupportedProtocolKeywords are protocol keywords the go-pcap expression
// parser accepts but its compiler does not turn into a matching test.
//
// Verified against github.com/packetcap/go-pcap@c86974bbfbcd: every keyword in
// filter/constants.go's subProtocols/protocols map parses, but
// filter/primitive.go's Compile only emits comparisons for tcp, udp, ip, ip6,
// arp and rarp. A primitive qualified by any other keyword — "icmp" being the
// obvious one a user would reach for — compiles to a program with no test in
// it at all, i.e. one that captures every packet on the interface. That is a
// silent correctness failure and a data-exposure footgun: the user believes
// they recorded ICMP and instead recorded the pod's entire network namespace,
// game traffic and control-plane traffic included. Rejecting these keywords
// outright is the only honest behaviour available to us; go-pcap gives no
// signal we could surface instead.
var unsupportedProtocolKeywords = map[string]struct{}{
	"aarp":    {},
	"ah":      {},
	"atalk":   {},
	"decnet":  {},
	"decnett": {},
	"esp":     {},
	"fddi":    {},
	"icmp":    {},
	"icmp6":   {},
	"igmp":    {},
	"igrp":    {},
	"ipx":     {},
	"iso":     {},
	"lat":     {},
	"modpl":   {},
	"morpc":   {},
	"netbeui": {},
	"pim":     {},
	"sca":     {},
	"stp":     {},
	"tr":      {},
	"vrrp":    {},
	"wlan":    {},
}

// CompileFilter validates and compiles a BPF filter expression.
// It returns an error immediately if the filter is invalid, ensuring
// invalid filters are rejected before any capture starts (FR-003).
// An empty expression returns an error (no implicit default here);
// callers must construct a default port-based filter if needed.
//
// Beyond syntax, this rejects expressions the underlying compiler cannot
// honour: a filter that would silently capture more than it says is worse than
// no capture at all, so anything that does not compile to a program which
// actually discards non-matching packets is refused.
func CompileFilter(expr string) (*Filter, error) {
	if expr == "" {
		return nil, fmt.Errorf("filter expression is empty")
	}

	if err := rejectUnsupportedKeywords(expr); err != nil {
		return nil, err
	}

	// Parse the tcpdump-style expression into an AST (github.com/packetcap/go-pcap/filter.Expression),
	// then compile that AST down to a BPF program. NewExpression never returns
	// an error itself (it returns nil only for an empty string, already
	// excluded above); syntax/semantic errors surface from the second Compile
	// call, which walks the AST and emits []bpf.Instruction.
	elem := filter.NewExpression(expr).Compile()
	instructions, err := elem.Compile()
	if err != nil {
		return nil, fmt.Errorf("invalid filter: %w", err)
	}

	if len(instructions) == 0 {
		return nil, fmt.Errorf("invalid filter: %q compiled to an empty program", expr)
	}
	if !testsPacketContents(instructions) {
		return nil, fmt.Errorf("invalid filter: %q compiled to a program with no packet test, so it would not restrict the capture at all", expr)
	}

	return &Filter{
		instructions: instructions,
		expr:         expr,
	}, nil
}

// rejectUnsupportedKeywords fails an expression that names a protocol the
// compiler cannot express. Words are taken between whitespace and parentheses,
// which is how go-pcap's own lexer splits them, and a leading backslash is
// stripped because "proto \icmp" is the escaped spelling of the same keyword.
func rejectUnsupportedKeywords(expr string) error {
	words := strings.FieldsFunc(expr, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '(' || r == ')'
	})
	for _, word := range words {
		normalized := strings.ToLower(strings.TrimLeft(word, `\`))
		if _, unsupported := unsupportedProtocolKeywords[normalized]; unsupported {
			return fmt.Errorf("invalid filter: protocol %q is not supported; use tcp/udp, host, net or port expressions", normalized)
		}
	}
	return nil
}

// testsPacketContents reports whether prog actually inspects the packet before
// deciding its fate. A BPF program without a conditional branch has a single
// execution path and therefore one verdict for every packet — it either keeps
// everything or drops everything — which is never what a capture filter is
// asked for. go-pcap produces exactly that shape for the qualifiers its
// compiler does not implement, so this is the structural backstop behind the
// keyword list above.
func testsPacketContents(prog []bpf.Instruction) bool {
	for _, ins := range prog {
		switch ins.(type) {
		case bpf.JumpIf, bpf.JumpIfX:
			return true
		}
	}
	return false
}

// Expression returns the original filter expression string.
func (f *Filter) Expression() string {
	return f.expr
}

// Instructions returns the compiled BPF program as gopacket/x-net's portable
// []bpf.Instruction representation. Callers that need the kernel-consumable
// []bpf.RawInstruction form (e.g. for afpacket.TPacket.SetBPF) must assemble
// it first via golang.org/x/net/bpf.Assemble.
func (f *Filter) Instructions() []bpf.Instruction {
	return f.instructions
}
