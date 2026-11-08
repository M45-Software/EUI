#!/usr/bin/env bash
# Run from any directory. Output includes CPU timings/allocations, not GPU FPS.
set -euo pipefail
cd "$(dirname "$0")/.."
output_dir="${1:-/tmp/eui-large-ui-benchmarks}"
mkdir -p "$output_dir"
output_dir="$(cd "$output_dir" && pwd)"
export CGO_ENABLED="${CGO_ENABLED:-0}"
runner=()
if [[ -z "${DISPLAY:-}" && "$(uname -s)" == Linux ]]; then
  runner=(xvfb-run -a)
fi
{
  go version
  go env GOOS GOARCH CGO_ENABLED
  printf 'BENCHTIME=%s COUNT=%s\n' "${BENCHTIME:-500ms}" "${COUNT:-5}"
} > "$output_dir/environment.txt"
for suite in BenchmarkLargeInterface BenchmarkLargeInterfaceFrames; do
  "${runner[@]}" go test -run '^$' -bench "^${suite}$" -benchmem \
    -benchtime="${BENCHTIME:-500ms}" -count="${COUNT:-5}" . | tee "$output_dir/$suite.txt"
done
