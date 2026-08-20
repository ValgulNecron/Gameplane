package gameproto

import (
	"bufio"
)

// DemoClassifier is a demonstration/reference implementation of the Classifier
// interface. It is intentionally inert — it always classifies connections as
// Unknown without consuming any bytes, and BuildStatusResponse/BuildDisconnect
// return errors or empty responses. DemoClassifier exists purely to prove that
// adding a new protocol requires only one implementation file (gameproto/demo.go)
// and one registry entry, with zero edits to shared code or the sentinel dispatcher.
//
// This stub can be used as a template when implementing a real protocol:
// copy this file, replace DemoClassifier with YourProtocolClassifier, and
// implement the four methods with real handshake parsing and response building.
type DemoClassifier struct{}

// Classify implements Classifier.Classify for the demo protocol.
// This stub always classifies as Unknown without reading any bytes,
// which ensures stream replay cannot fail — no bytes are consumed.
func (d *DemoClassifier) Classify(br *bufio.Reader) (*ClassificationResult, error) {
	return &ClassificationResult{
		Kind:     Unknown,
		Consumed: nil,
		Detail:   nil,
	}, nil
}

// SupportsStatusPing implements Classifier.SupportsStatusPing for the demo protocol.
// The demo protocol does not support out-of-band status pings.
func (d *DemoClassifier) SupportsStatusPing() bool {
	return false
}

// BuildStatusResponse implements Classifier.BuildStatusResponse for the demo protocol.
// Since SupportsStatusPing returns false, this method always returns an error.
func (d *DemoClassifier) BuildStatusResponse(payload string) ([]byte, error) {
	return nil, ErrStatusPingUnsupported
}

// BuildDisconnect implements Classifier.BuildDisconnect for the demo protocol.
// This stub has no wire format; it returns empty bytes and no error.
func (d *DemoClassifier) BuildDisconnect(reason string) ([]byte, error) {
	return nil, nil
}
