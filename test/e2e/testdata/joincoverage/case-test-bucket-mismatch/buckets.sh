#!/bin/bash
# Minimal buckets.sh for fixture testing
case "${1:-}" in
list)
    case "${2:-}" in
    bot-fast)
        # bot-fast bucket intentionally does NOT include the test
        # to trigger the cross-reference check failure
        ;;
    *)
        echo "Unknown bucket: $2" >&2
        exit 1
        ;;
    esac
    ;;
*)
    echo "Unknown subcommand" >&2
    exit 1
    ;;
esac
