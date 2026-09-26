package main

import "testing"

// TestDefaultVolumeBudgetBytes_MatchesOperator pins defaultVolumeBudgetBytes
// to the value it must keep matching by hand: the operator's
// captureVolumeBudgetBytes in
// operator/internal/controller/gameserver_controller.go (1Gi "captures"
// emptyDir SizeLimit minus a 10% safety margin, i.e. 966367642 bytes). The
// two constants can't share Go code across modules, so this test exists to
// catch an accidental edit to either side going unnoticed (see
// defaultVolumeBudgetBytes' doc comment in main.go).
func TestDefaultVolumeBudgetBytes_MatchesOperator(t *testing.T) {
	const wantOperatorCaptureVolumeBudgetBytes = 966367642
	if defaultVolumeBudgetBytes != wantOperatorCaptureVolumeBudgetBytes {
		t.Errorf("defaultVolumeBudgetBytes = %d, want %d (must match captureVolumeBudgetBytes in operator/internal/controller/gameserver_controller.go)",
			defaultVolumeBudgetBytes, wantOperatorCaptureVolumeBudgetBytes)
	}
}
