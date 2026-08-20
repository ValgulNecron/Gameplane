package gameproto

import "sort"

// classifierRegistry is the static, compile-time registry mapping protocol names
// to Classifier implementations. Populated with explicit entries for each supported
// protocol. Duplicate keys are a Go compile error.
//
// The registry is a bare package-level map, not a struct wrapper. This design:
//   - Makes the registration set greppable and auditable in one place (source of truth).
//   - Prevents duplicate registrations (Go compiler error on duplicate literal keys).
//   - Requires no initialization code (map literal is assigned at package load time).
var classifierRegistry = map[string]Classifier{
	"minecraft": &MinecraftClassifier{},
	"terraria":  &TerrariaClassifier{},
	"demo":      &DemoClassifier{},
}

// Lookup retrieves a Classifier by protocol name from the registry.
// Returns (classifier, true) if found, (nil, false) if not found.
// Never panics on unknown names; caller must handle lookup miss gracefully.
func Lookup(name string) (Classifier, bool) {
	classifier, ok := classifierRegistry[name]
	return classifier, ok
}

// ListRegistered returns a sorted slice of all registered protocol names for
// debugging and audit purposes. Useful for logging or verifying registry
// completeness at startup.
func ListRegistered() []string {
	names := make([]string, 0, len(classifierRegistry))
	for name := range classifierRegistry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
