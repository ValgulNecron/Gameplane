// Package notify delivers cluster-event notifications (server health,
// backup/restore outcomes) to admin-configured sinks: Discord, Slack,
// SMTP, or a generic webhook. It is the UX-layer counterpart of the
// audit sinks — the operator stays authoritative for reconciliation;
// this package only mirrors CRD status transitions it observes and
// never feeds back into them.
package notify

// EventType names one notifiable transition. The set is closed: the
// config validator, the sink event filters, and the web panel all key
// off it, so adding a type means touching all three.
type EventType string

const (
	EventServerUnhealthy  EventType = "server.unhealthy"
	EventServerRecovered  EventType = "server.recovered"
	EventBackupFailed     EventType = "backup.failed"
	EventBackupSucceeded  EventType = "backup.succeeded"
	EventRestoreFailed    EventType = "restore.failed"
	EventRestoreSucceeded EventType = "restore.succeeded"
)

// AllEvents lists every event type in display order.
var AllEvents = []EventType{
	EventServerUnhealthy,
	EventServerRecovered,
	EventBackupFailed,
	EventBackupSucceeded,
	EventRestoreFailed,
	EventRestoreSucceeded,
}

// ValidEvent reports whether s names a known event type.
func ValidEvent(s string) bool {
	for _, t := range AllEvents {
		if string(t) == s {
			return true
		}
	}
	return false
}

// DefaultOn reports whether a sink with no explicit events filter
// receives t: failures alert by default, and server.recovered pairs
// with server.unhealthy so an alerted outage also reports its end.
// Success chatter (backup/restore succeeded) is opt-in.
func DefaultOn(t EventType) bool {
	switch t {
	case EventServerUnhealthy, EventServerRecovered,
		EventBackupFailed, EventRestoreFailed:
		return true
	default:
		return false
	}
}
