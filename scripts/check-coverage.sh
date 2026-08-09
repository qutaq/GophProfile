#!/usr/bin/env bash
set -euo pipefail

OUT=coverage.out
go test ./internal/... -coverprofile="$OUT" -covermode=atomic
TOTAL=$(go tool cover -func="$OUT" | awk '/total:/ {print $3}' | tr -d '%')
echo "total: ${TOTAL}%"
MIN=50
awk -v t="$TOTAL" -v m="$MIN" 'BEGIN { if (t+0 < m) { printf("coverage %.1f%% is below required %d%%\n", t, m); exit 1 } }'
echo "coverage OK (>= ${MIN}%)"
