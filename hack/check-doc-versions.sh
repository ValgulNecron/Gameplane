#!/usr/bin/env bash
# check-doc-versions.sh: Verify that all audited documentation files have version strings that match the current appVersion.
#
# Purpose:
#   Ensures compliance with SC-005 (Version String Audit Standard) from feature 012
#   (docs-refresh-and-outreach). Validates that every product documentation claim about
#   Gameplane version matches the current appVersion from charts/gameplane/Chart.yaml,
#   or is explicitly marked as an example version or historical reference.
#
# Audited Files (18):
#   README.md, docs/architecture.md, docs/contributing.md, docs/dependencies.md,
#   docs/game-coverage.md, docs/install.md, docs/key-rotation.md,
#   docs/module-authoring.md, docs/networking.md, docs/notifications.md,
#   docs/oidc.md, docs/roadmap.md, docs/security.md, docs/tunnels.md,
#   audit-syslog-bridge/README.md, mcp-server/README.md,
#   telemetry-receiver/README.md, docs/comparison-sources.md
#
# Allowlist Rules (OD-1, OD-14):
#   A version string v?0\.[0-9]+\.[0-9]+-beta\.[0-9]+ passes if:
#   1. It matches the current appVersion (with or without leading 'v'), OR
#   2. The line contains "(example version)", "(example)", or "(placeholder)", OR
#   3. The line or ±2 lines contain "# Example:" or "<!-- Example -->", OR
#   4. The line contains "<!-- doc-versions: historical -->" (OD-14)
#
# Exit codes:
#   0 = all version strings match current appVersion or are allowlisted
#   1 = one or more stale version strings found, or audited files missing

set -euo pipefail

# Navigate to repo root (one level up from script's directory)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$REPO_ROOT"

# Check that Chart.yaml exists and is readable
if [[ ! -f charts/gameplane/Chart.yaml ]] || [[ ! -r charts/gameplane/Chart.yaml ]]; then
    echo "✗ Error: charts/gameplane/Chart.yaml not found or unreadable"
    exit 1
fi

# Extract appVersion from Chart.yaml (line 6: appVersion: "X.Y.Z-beta.N")
# The regex extracts the quoted or unquoted version string
current_version=$(sed -n '6p' charts/gameplane/Chart.yaml | sed 's/^appVersion:[[:space:]]*"\?//;s/"[[:space:]]*$//')

if [[ -z "$current_version" ]]; then
    echo "✗ Error: Could not extract appVersion from charts/gameplane/Chart.yaml:6"
    exit 1
fi

# Normalize: appVersion in chart is unquoted (0.2.0-beta.8)
# We'll accept matches with or without leading 'v'
current_version_pattern="(v)?${current_version//./\\.}"

# List of 18 audited files
audited_files=(
    "README.md"
    "docs/architecture.md"
    "docs/contributing.md"
    "docs/dependencies.md"
    "docs/game-coverage.md"
    "docs/install.md"
    "docs/key-rotation.md"
    "docs/module-authoring.md"
    "docs/networking.md"
    "docs/notifications.md"
    "docs/oidc.md"
    "docs/roadmap.md"
    "docs/security.md"
    "docs/tunnels.md"
    "audit-syslog-bridge/README.md"
    "mcp-server/README.md"
    "telemetry-receiver/README.md"
    "docs/comparison-sources.md"
)

# Validate files and count
failed_files=()
failed_count=0

for file in "${audited_files[@]}"; do
    if [[ ! -f "$file" ]]; then
        echo "✗ $file: missing audited file"
        failed_files+=("$file")
        failed_count=$((failed_count + 1))
    fi
done

# If any files are missing, report and exit early
if [[ ${#failed_files[@]} -gt 0 ]]; then
    if [[ $failed_count -eq 1 ]]; then
        echo "✗ 1 audited file is missing"
    else
        echo "✗ $failed_count audited files are missing"
    fi
    exit 1
fi

# Process each file for version string matches
version_errors=()

for file in "${audited_files[@]}"; do
    if [[ ! -r "$file" ]]; then
        continue
    fi

    # Grep for version pattern v?0\.[0-9]+\.[0-9]+-beta\.[0-9]+
    # -n includes line numbers
    while IFS= read -r line_data; do
        # Parse line_data format: "line_number:content"
        line_num="${line_data%%:*}"
        line_content="${line_data#*:}"

        # Extract all version matches on this line (non-greedy version literals)
        # Use sed to extract matches
        version_matches=$(echo "$line_content" | grep -oE 'v?0\.[0-9]\.[0-9]-beta\.[0-9]+' || true)

        if [[ -z "$version_matches" ]]; then
            continue
        fi

        # For each match on this line
        while IFS= read -r match; do
            [[ -z "$match" ]] && continue

            # Check if match equals current version (with or without leading 'v')
            match_normalized="${match#v}"
            current_normalized="${current_version#v}"

            if [[ "$match_normalized" == "$current_normalized" ]]; then
                # Match is current version; pass
                continue
            fi

            # Match is not current version; check for allowlist markers
            is_allowlisted=0

            # Check current line for allowlist markers
            if echo "$line_content" | grep -q '(example version)\|(example)\|(placeholder)\|# Example:\|<!-- Example -->\|<!-- doc-versions: historical -->'; then
                is_allowlisted=1
            fi

            # Check ±2 lines for example markers (not historical marker, which is line-specific)
            if [[ $is_allowlisted -eq 0 ]]; then
                # Check lines above and below
                for offset in -2 -1 1 2; do
                    check_line=$((line_num + offset))
                    if [[ $check_line -gt 0 ]]; then
                        # Use sed to get the Nth line
                        context_line=$(sed -n "${check_line}p" "$file" 2>/dev/null || true)
                        if echo "$context_line" | grep -q '# Example:\|<!-- Example -->'; then
                            is_allowlisted=1
                            break
                        fi
                    fi
                done
            fi

            # If not allowlisted, record as error
            if [[ $is_allowlisted -eq 0 ]]; then
                version_errors+=("$file:$line_num: $match (current appVersion is $current_version)")
            fi
        done <<< "$version_matches"
    done < <(grep -nE 'v?0\.[0-9]\.[0-9]-beta\.[0-9]+' "$file" || true)
done

# Report results
file_count=${#audited_files[@]}
if [[ ${#version_errors[@]} -eq 0 ]]; then
    echo "✓ Checked $file_count files: all Gameplane version strings match appVersion $current_version or are allowlisted"
    exit 0
else
    # Report each error
    for error in "${version_errors[@]}"; do
        echo "✗ $error"
    done

    error_count=${#version_errors[@]}
    # Count unique files with errors
    unique_files=$(printf '%s\n' "${version_errors[@]}" | cut -d: -f1 | sort -u | wc -l)

    if [[ $error_count -eq 1 ]]; then
        echo "✗ 1 stale version string across 1 file"
    else
        echo "✗ $error_count stale version string(s) across $unique_files file(s)"
    fi
    exit 1
fi
