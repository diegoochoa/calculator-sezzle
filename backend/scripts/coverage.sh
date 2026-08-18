#!/bin/sh
#
# Fails the build when statement coverage falls below the threshold.
# Usage: scripts/coverage.sh [profile]   (default: coverage.out)
#
# POSIX sh rather than bash: this also runs inside the Alpine build stage, which
# ships busybox ash and no bash at all.
#
set -eu

THRESHOLD="${COVERAGE_THRESHOLD:-85}"
PROFILE="${1:-coverage.out}"

if [ ! -f "$PROFILE" ]; then
  echo "coverage: no profile at $PROFILE — run 'make cover' first" >&2
  exit 1
fi

total="$(go tool cover -func="$PROFILE" | awk '/^total:/ { gsub(/%/, "", $3); print $3 }')"

if [ -z "$total" ]; then
  echo "coverage: could not read a total from $PROFILE" >&2
  exit 1
fi

# awk rather than the shell, which cannot compare floats.
if awk -v total="$total" -v threshold="$THRESHOLD" 'BEGIN { exit !(total < threshold) }'; then
  echo "coverage: ${total}% is below the ${THRESHOLD}% threshold" >&2
  exit 1
fi

echo "coverage: ${total}% (threshold ${THRESHOLD}%)"
