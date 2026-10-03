package gatewayprotocol

import (
	"strings"
	"testing"
)

func TestCapturePathBindsBothIdentities(t *testing.T) {
	target := CaptureTarget{Target: Target{Cluster: "remote", Namespace: "games", Name: "same-name", UID: "server-uid"}, Capture: "cap-one", CaptureUID: "capture-uid"}
	path, err := CapturePath(target)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseCapturePath(path)
	if err != nil || parsed != target {
		t.Fatalf("round trip: %+v %v", parsed, err)
	}
	for _, bad := range []string{path + "/extra", path + "?url=http://elsewhere", strings.Replace(path, "cap-one", "..", 1), strings.Replace(path, "capture-uid", "", 1), strings.Replace(path, "cap-one", "%2fetc", 1), strings.Replace(path, "/uids/capture-uid", "", 1)} {
		if _, err := ParseCapturePath(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}
