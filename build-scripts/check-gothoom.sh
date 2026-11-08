#!/usr/bin/env bash
# Test a goThoom checkout against this EUI tree without modifying the client.
set -euo pipefail

eui_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
client_root=${1:?Usage: check-gothoom.sh /path/to/goThoom}
if [[ -f "$client_root/source/go.mod" ]]; then
  client_source="$client_root/source"
else
  client_source="$client_root"
fi
[[ -f "$client_source/go.mod" ]] || { echo "No client go.mod found" >&2; exit 1; }
client_source=$(cd -- "$client_source" && pwd)
client_repo=$(dirname -- "$client_source")
client_check_dir=$(mktemp -d)
trap 'rm -rf -- "$client_check_dir"' EXIT
cp -R "$client_source" "$client_check_dir/source"
# Client tests also read these checked-in fixtures outside the Go module.
if [[ -d "$client_repo/docs" ]]; then
  cp -R "$client_repo/docs" "$client_check_dir/docs"
fi
for fixture in bard-instrument-audition.mid bard-instrument-audition.wav bard-instrument-audition-quicktime.wav; do
  if [[ -f "$client_repo/$fixture" ]]; then
    cp "$client_repo/$fixture" "$client_check_dir/$fixture"
  fi
done
rm -rf -- "$client_check_dir/source/eui"
python3 - "$client_check_dir/source" <<'PY'
import sys
from pathlib import Path
root = Path(sys.argv[1])
for file in root.rglob('*.go'):
    original = file.read_text()
    rewritten = original.replace('gothoom/eui', 'github.com/Distortions81/EUI')
    if rewritten != original:
        file.write_text(rewritten)
PY
cd "$client_check_dir/source"
export GOWORK=off
export CGO_ENABLED=0
go mod edit -require=github.com/Distortions81/EUI@v0.0.0 \
  "-replace=github.com/Distortions81/EUI=$eui_root"
go mod tidy
go test ./...
go build -o "$client_check_dir/gothoom" .
