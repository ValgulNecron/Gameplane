package handlers

import "golang.org/x/mod/semver"

// semverDescending returns true when a should sort before b in
// descending semver order. Used to merge ModuleSource catalogs that
// each carry their own (already sorted) version slices.
func semverDescending(a, b string) bool {
	return semver.Compare("v"+a, "v"+b) > 0
}
