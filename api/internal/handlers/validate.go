package handlers

import "regexp"

// Shared regex patterns for input validation across handlers.
// These patterns enforce strict DNS/identifier naming rules consistently
// across the API surface.

// dnsLabelRE matches an RFC1123 label: lowercase alphanumeric and dashes,
// no leading/trailing dash, 1-63 chars. Used for K8s namespace and Secret
// name fields where the value will eventually be passed to the API server.
var dnsLabelRE = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`)

// identifierRE constrains identifiers (usernames, role names) to a
// conservative alphanumeric set with limited special chars. This stops
// homoglyph tricks and keeps names URL-safe and log-friendly.
var identifierRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$`)
