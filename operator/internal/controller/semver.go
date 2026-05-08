package controller

import "golang.org/x/mod/semver"

// semverDescending returns true when a should sort before b in
// descending semver order. Plain "1.2.3" tags are compared with a "v"
// prefix because golang.org/x/mod/semver requires it.
func semverDescending(a, b string) bool {
	return semver.Compare("v"+a, "v"+b) > 0
}
