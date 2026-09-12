package common

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// dns1123LabelRegex matches valid Kubernetes DNS-1123 label names.
	dns1123LabelRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

	// canonicalCategories is the authoritative taxonomy for Gameplane module catalog filter chips.
	canonicalCategories = []string{
		"Survival",
		"Sandbox",
		"Shooter",
		"Simulation",
		"Building",
		"Adventure",
		"Horror",
		"Co-op",
		"PvP",
		"Modded",
		"Creative",
	}

	canonicalCategoriesMap = func() map[string]string {
		m := make(map[string]string, len(canonicalCategories))
		for _, cat := range canonicalCategories {
			m[strings.ToLower(cat)] = cat
		}
		return m
	}()
)

// ValidateModuleName verifies that name conforms to RFC 1123 DNS label requirements.
func ValidateModuleName(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("module name cannot be empty")
	}
	if len(name) > 63 {
		return fmt.Errorf("module name %q exceeds maximum length of 63 characters (%d chars)", name, len(name))
	}
	if !dns1123LabelRegex.MatchString(name) {
		return fmt.Errorf("module name %q is invalid: must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character (regex: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$)", name)
	}
	return nil
}

// CanonicalCategories returns a slice of the recognized catalog categories.
func CanonicalCategories() []string {
	cp := make([]string, len(canonicalCategories))
	copy(cp, canonicalCategories)
	return cp
}

// IsCanonicalCategory returns true if the category matches the canonical taxonomy (case-insensitive).
func IsCanonicalCategory(cat string) bool {
	_, ok := canonicalCategoriesMap[strings.ToLower(strings.TrimSpace(cat))]
	return ok
}

// NormalizeCategory returns the properly-cased canonical category name if recognized, or the original trimmed string.
func NormalizeCategory(cat string) string {
	trimmed := strings.TrimSpace(cat)
	if canonical, ok := canonicalCategoriesMap[strings.ToLower(trimmed)]; ok {
		return canonical
	}
	return trimmed
}
