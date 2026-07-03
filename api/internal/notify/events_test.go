package notify

import "testing"

func TestValidEvent(t *testing.T) {
	for _, ev := range AllEvents {
		if !ValidEvent(string(ev)) {
			t.Errorf("ValidEvent(%q) = false, want true", ev)
		}
	}
	for _, s := range []string{"", "server.rebooted", "backup", "SERVER.UNHEALTHY"} {
		if ValidEvent(s) {
			t.Errorf("ValidEvent(%q) = true, want false", s)
		}
	}
}

func TestDefaultOn(t *testing.T) {
	want := map[EventType]bool{
		EventServerUnhealthy:  true,
		EventServerRecovered:  true,
		EventBackupFailed:     true,
		EventBackupSucceeded:  false,
		EventRestoreFailed:    true,
		EventRestoreSucceeded: false,
	}
	for _, ev := range AllEvents {
		if got := DefaultOn(ev); got != want[ev] {
			t.Errorf("DefaultOn(%q) = %v, want %v", ev, got, want[ev])
		}
	}
}
