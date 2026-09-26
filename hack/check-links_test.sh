#!/usr/bin/env bash
# check-links_test.sh: self-test entry point for hack/check-links.sh's
# github_slug anchor-slug conversion, including the Unicode-punctuation
# cases from F-236 (e.g. "→", "—" headings). Not wired into `make
# check-links` or CI — run manually when touching github_slug.
#
# Usage: hack/check-links_test.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "${SCRIPT_DIR}/check-links.sh" --self-test
