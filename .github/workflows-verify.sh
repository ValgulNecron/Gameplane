#!/usr/bin/env bash
# Verify the repository's GitHub Actions configuration against the hardening
# rules in specs/008-hardened-github-actions/.
#
# Usage:
#   .github/workflows-verify.sh verify              # run every rule
#   .github/workflows-verify.sh verify --rule R4    # run one rule
#   .github/workflows-verify.sh list                # list registered rules
#
# Exits 0 when every rule passes, 1 when any rule fails, 2 on a usage or
# environment error. Follows the `verify` subcommand convention of
# test/e2e/buckets.sh so the CI wiring and the muscle memory match.

set -euo pipefail

SCRIPT_DIR="$(CDPATH="" cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(CDPATH="" cd -- "$SCRIPT_DIR/.." && pwd)"
RULES_DIR="$SCRIPT_DIR/verify-rules"

usage() {
	cat <<'EOF'
usage: workflows-verify.sh <command> [options]

commands:
  verify            run the rules and report pass/fail
  list              list registered rules and exit

options:
  --rule <ID>       run only the named rule (e.g. --rule R4)
  -h, --help        show this message
EOF
}

COMMAND="${1:-}"
[ $# -gt 0 ] && shift || true

ONLY_RULE=""
while [ $# -gt 0 ]; do
	case "$1" in
	--rule)
		ONLY_RULE="${2:-}"
		if [ -z "$ONLY_RULE" ]; then
			echo "workflows-verify: --rule needs an argument" >&2
			exit 2
		fi
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "workflows-verify: unknown option: $1" >&2
		usage >&2
		exit 2
		;;
	esac
done

case "$COMMAND" in
verify | list) ;;
"" | -h | --help)
	usage
	exit 0
	;;
*)
	echo "workflows-verify: unknown command: $COMMAND" >&2
	usage >&2
	exit 2
	;;
esac

if ! command -v python3 >/dev/null 2>&1; then
	echo "workflows-verify: python3 is required but not on PATH" >&2
	exit 2
fi

if [ ! -d "$RULES_DIR" ]; then
	echo "workflows-verify: no rules directory at $RULES_DIR" >&2
	exit 2
fi

COMMAND="$COMMAND" ONLY_RULE="$ONLY_RULE" REPO_ROOT="$REPO_ROOT" \
	python3 - "$RULES_DIR" <<'PYEOF'
import importlib.util
import os
import pathlib
import sys

rules_dir = pathlib.Path(sys.argv[1])
command = os.environ.get("COMMAND", "verify")
only_rule = (os.environ.get("ONLY_RULE") or "").strip()
repo_root = os.environ["REPO_ROOT"]

sys.path.insert(0, str(rules_dir))
import _common  # noqa: E402


def load_rules():
    """Import every r<n>_*.py in the rules directory, ordered numerically."""
    modules = []
    for path in sorted(rules_dir.glob("r*.py")):
        if path.name.startswith("_"):
            continue
        spec = importlib.util.spec_from_file_location(path.stem, path)
        if spec is None or spec.loader is None:
            continue
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        for attr in ("RULE_ID", "DESCRIPTION", "check"):
            if not hasattr(module, attr):
                sys.stderr.write(
                    f"workflows-verify: {path.name} does not expose {attr}; "
                    "see the rule module contract in _common.py\n"
                )
                raise SystemExit(2)
        modules.append(module)

    def sort_key(mod):
        digits = "".join(c for c in mod.RULE_ID if c.isdigit())
        return int(digits) if digits else 0

    return sorted(modules, key=sort_key)


rules = load_rules()

if command == "list":
    for rule in rules:
        print(f"{rule.RULE_ID}\t{rule.DESCRIPTION}")
    raise SystemExit(0)

if only_rule:
    wanted = only_rule.upper()
    rules = [r for r in rules if r.RULE_ID.upper() == wanted]
    if not rules:
        sys.stderr.write(f"workflows-verify: no such rule: {only_rule}\n")
        raise SystemExit(2)

if not rules:
    print("workflows-verify: no rules registered")
    raise SystemExit(0)

ctx = _common.build_ctx(repo_root)

failed = 0
for rule in rules:
    try:
        violations = rule.check(ctx)
    except Exception as exc:  # a crashing rule is a failing rule, never a pass
        print(f"{rule.RULE_ID} ERROR: {rule.DESCRIPTION}")
        print(f"  rule raised {type(exc).__name__}: {exc}")
        failed += 1
        continue

    if violations:
        failed += 1
        print(f"{rule.RULE_ID} FAIL: {rule.DESCRIPTION} ({len(violations)} violation(s))")
        for violation in violations:
            print(violation.render())
    else:
        summary = getattr(rule, "summary", lambda _ctx: "")(ctx)
        suffix = f" -- {summary}" if summary else ""
        print(f"{rule.RULE_ID} pass: {rule.DESCRIPTION}{suffix}")

print()
if failed:
    print(f"workflows-verify: {failed} of {len(rules)} rule(s) FAILED")
    raise SystemExit(1)
print(f"workflows-verify: all {len(rules)} rule(s) passed")
raise SystemExit(0)
PYEOF
