package joindepth

import (
	"fmt"
	"strings"
)

// VerdictResult represents the outcome classification of a probe run.
type VerdictResult string

const (
	// PASS means the probe reached the expected depth (or, under -expect-fail, correctly failed).
	PASS VerdictResult = "PASS"
	// FAIL_WRONG_DEPTH means the probe connected to a live listener but did not reach the expected depth.
	FAIL_WRONG_DEPTH VerdictResult = "FAIL_WRONG_DEPTH"
	// FAIL_TRANSPORT means the probe encountered a transport failure (connection refused, timeout, etc.).
	FAIL_TRANSPORT VerdictResult = "FAIL_TRANSPORT"
	// FAIL_INTERNAL_ERROR means the probe encountered an internal error (bad flags, panic, unusable environment).
	FAIL_INTERNAL_ERROR VerdictResult = "FAIL_INTERNAL_ERROR"
	// FAIL_NEGATIVE_CONTROL_REACHED means the probe was run with -expect-fail and unexpectedly reached the expected depth.
	FAIL_NEGATIVE_CONTROL_REACHED VerdictResult = "FAIL_NEGATIVE_CONTROL_REACHED"
)

// ProbeVerdict is the structured result a per-game probe binary reports back to the E2E test harness.
type ProbeVerdict struct {
	// ReachedDepth is the depth actually achieved before success or failure.
	ReachedDepth JoinDepth
	// Detail is a human-readable name of the server-originated artifact proving the depth.
	// MANDATORY for JOINED (e.g., "login success for user bot#1", "WorldData frame received").
	// OPTIONAL for PARTIAL and QUERY (but highly recommended for debugging).
	// For transport failures (exit code 3), this should describe the connection-level event.
	Detail string
	// Err is the transport or protocol error, if any. Distinct from "reached a lower depth than expected."
	// Set when exit code is non-zero.
	Err error
}

// Encode produces the single machine-readable VERDICT line as per the probe-cli.md contract.
// The format is:
//
//	VERDICT	<result>	<depth>	<depth_evidence>
//
// Tab-separated fields. Returns an error if the invariant is violated (JOINED without Detail).
func (pv *ProbeVerdict) Encode() (string, error) {
	// Invariant: JOINED must have Detail.
	if pv.ReachedDepth == JOINED && pv.Detail == "" {
		return "", fmt.Errorf("invalid verdict: JOINED depth requires non-empty Detail (server-originated artifact evidence)")
	}

	result := VerdictResultFromVerdict(pv)
	depthStr := pv.ReachedDepth.String()

	// Format the line as tab-separated.
	return fmt.Sprintf("VERDICT\t%s\t%s\t%s", result, depthStr, pv.Detail), nil
}

// VerdictResultFromVerdict maps a ProbeVerdict to its VerdictResult classification.
func VerdictResultFromVerdict(pv *ProbeVerdict) VerdictResult {
	// If Err is set, classify based on the error type.
	if pv.Err != nil {
		// Heuristic: if the error message contains transport-level keywords, classify as transport.
		errMsg := pv.Err.Error()
		if strings.Contains(errMsg, "connection refused") ||
			strings.Contains(errMsg, "Dial timeout") ||
			strings.Contains(errMsg, "timeout") ||
			strings.Contains(errMsg, "connection never established") ||
			strings.Contains(errMsg, "DNS failure") ||
			strings.Contains(errMsg, "No response") ||
			strings.Contains(errMsg, "connection reset") {
			return FAIL_TRANSPORT
		}
		// Default to internal error if unclassified.
		return FAIL_INTERNAL_ERROR
	}

	// No error; verdict is based on depth matching.
	// (This function is primarily for classification; the calling test harness
	// will determine PASS vs FAIL based on the expected depth assertion.)
	return PASS
}

// ExitCodeFromVerdict maps a ProbeVerdict to its POSIX exit code.
func ExitCodeFromVerdict(pv *ProbeVerdict, expectedDepth JoinDepth, expectFail bool) int {
	if expectFail {
		// Under -expect-fail, invert the logic.
		if pv.ReachedDepth == expectedDepth && pv.Err == nil {
			// Probe reached the expected depth when it should not have. It connected
			// successfully (Err is nil), so exit 2: connected but wrong depth (from the
			// negative control's perspective, reaching the expected depth is "wrong").
			return 2
		}
		// Probe correctly failed to reach the depth.
		return 0
	}

	// Normal (positive control) case.
	if pv.ReachedDepth == expectedDepth && pv.Err == nil {
		// Success: reached the expected depth.
		return 0
	}

	// Classify the failure.
	if pv.Err != nil {
		errMsg := pv.Err.Error()
		if strings.Contains(errMsg, "connection refused") ||
			strings.Contains(errMsg, "Dial timeout") ||
			strings.Contains(errMsg, "timeout") ||
			strings.Contains(errMsg, "connection never established") ||
			strings.Contains(errMsg, "DNS failure") ||
			strings.Contains(errMsg, "No response") ||
			strings.Contains(errMsg, "connection reset") {
			return 3 // Transport failure.
		}
		return 1 // Internal error.
	}

	// Connected but wrong depth.
	return 2
}

// OutcomeFromExitCode maps an exit code back to a human-readable outcome description.
func OutcomeFromExitCode(exitCode int) string {
	switch exitCode {
	case 0:
		return "success: reached expected depth (or correctly failed)"
	case 1:
		return "internal error: bad flags, panic, or unusable environment"
	case 2:
		return "connected but wrong depth: protocol or join handshake broken"
	case 3:
		return "transport failure: nothing listening, connection refused, DNS failure, or timeout"
	default:
		return "unknown exit code"
	}
}

// ParseVerdict consumes a VERDICT line and returns a ProbeVerdict.
// Expected format: "VERDICT\t<result>\t<depth>\t<detail>"
func ParseVerdict(line string) (*ProbeVerdict, error) {
	fields := strings.Split(line, "\t")
	if len(fields) < 4 {
		return nil, fmt.Errorf("invalid verdict line: not enough fields (expected 4, got %d): %q", len(fields), line)
	}

	// Field 0: VERDICT prefix
	if fields[0] != "VERDICT" {
		return nil, fmt.Errorf("invalid verdict line: expected prefix 'VERDICT', got %q", fields[0])
	}

	// Field 1: result (ignored for now; callers can infer from ReachedDepth and Err)
	// Field 2: depth
	depthStr := fields[2]
	depth, err := Parse(depthStr)
	if err != nil {
		return nil, fmt.Errorf("invalid verdict line: parsing depth: %w", err)
	}

	// Field 3: detail
	detail := fields[3]

	// Invariant: JOINED must have non-empty detail.
	if depth == JOINED && detail == "" {
		return nil, fmt.Errorf("invalid verdict line: JOINED depth requires non-empty detail evidence")
	}

	return &ProbeVerdict{
		ReachedDepth: depth,
		Detail:       detail,
		Err:          nil, // Err is not encoded in the VERDICT line; it's logged separately.
	}, nil
}

// String returns a human-readable string representation of the ProbeVerdict.
// This is distinct from Encode(), which produces the machine-readable VERDICT line.
func (pv *ProbeVerdict) String() string {
	var sb strings.Builder
	sb.WriteString("ProbeVerdict{")
	sb.WriteString("ReachedDepth=")
	sb.WriteString(pv.ReachedDepth.String())
	sb.WriteString(", Detail=")
	sb.WriteString(pv.Detail)
	if pv.Err != nil {
		sb.WriteString(", Err=")
		sb.WriteString(pv.Err.Error())
	}
	sb.WriteString("}")
	return sb.String()
}
