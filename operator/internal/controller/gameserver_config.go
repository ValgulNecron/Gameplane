package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
)

// configHashAnnotation is stamped on the pod template whenever the
// GameServer materializes any config. It covers every resolved value,
// so changes that would otherwise not alter the pod spec (e.g. a
// Secret-backed value) still roll the StatefulSet.
const configHashAnnotation = "kestrel.gg/config-hash"

// materializedConfig is the result of resolving GameServer.spec.config
// against the referenced template's configSchema.
type materializedConfig struct {
	// env holds the config-derived env vars for the game container, in
	// schema order. Precedence: template env < env < GameServer.spec.env.
	env []corev1.EnvVar
	// hash fingerprints all resolved values for configHashAnnotation.
	// Empty when no config materialized.
	hash string
}

// materializeConfig validates spec.config against the template's
// configSchema and resolves it into container env vars. Defaults are
// applied for absent keys (keeping `kubectl apply` behavior identical
// to the wizard, which pre-fills them); empty optional values are
// skipped entirely so games treat them as unset. Any violation is an
// error — the caller fails the GameServer rather than silently
// dropping user intent.
func materializeConfig(
	gs *kestrelv1alpha1.GameServer, tmpl *kestrelv1alpha1.GameTemplate,
) (*materializedConfig, error) {
	schema := tmpl.Spec.ConfigSchema
	known := make(map[string]bool, len(schema))
	names := make([]string, 0, len(schema))
	for _, f := range schema {
		known[f.Name] = true
		names = append(names, f.Name)
	}
	var unknown []string
	for k := range gs.Spec.Config {
		if !known[k] {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("unknown config keys %s: template %q accepts [%s]",
			strings.Join(unknown, ", "), tmpl.Name, strings.Join(names, ", "))
	}

	mc := &materializedConfig{}
	var hashParts []string
	for _, f := range schema {
		val, set := gs.Spec.Config[f.Name]
		if !set {
			val = f.Default
		}
		if val == "" {
			if f.Required {
				return nil, fmt.Errorf("required config field %q has no value and no default", f.Name)
			}
			// Unset optional field: no env var at all, so the game
			// falls back to its own default instead of seeing "".
			continue
		}
		// Type/Target carry kubebuilder defaults ("string"/"env") that
		// only the API server applies, so treat "" as the default here.
		switch f.Type {
		case "int":
			if _, err := strconv.ParseInt(val, 10, 64); err != nil {
				return nil, fmt.Errorf("config field %q: %q is not an integer", f.Name, val)
			}
		case "bool":
			if _, err := strconv.ParseBool(val); err != nil {
				return nil, fmt.Errorf("config field %q: %q is not a boolean", f.Name, val)
			}
		case "enum":
			if !slices.Contains(f.Enum, val) {
				return nil, fmt.Errorf("config field %q: %q is not one of [%s]",
					f.Name, val, strings.Join(f.Enum, ", "))
			}
		}
		if f.Target == "file" {
			return nil, fmt.Errorf("config field %q targets a file; file targets are not implemented yet", f.Name)
		}
		mc.env = append(mc.env, corev1.EnvVar{Name: f.Name, Value: val})
		hashParts = append(hashParts, f.Name+"="+val)
	}
	if len(hashParts) > 0 {
		sum := sha256.Sum256([]byte(strings.Join(hashParts, "\n")))
		mc.hash = hex.EncodeToString(sum[:])
	}
	return mc, nil
}
