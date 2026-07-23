#!/usr/bin/env bash
set -u

if [ $# -lt 1 ] || [ -z "$1" ]; then
  echo "usage: $0 <phase-name> [iteration-label]" >&2
  exit 64
fi

if ! command -v snyk >/dev/null 2>&1; then
  echo "snyk CLI not found in PATH" >&2
  exit 127
fi

phase="$1"
iter="${2:-}"
target_ref="$phase"
if [ -n "$iter" ]; then
  target_ref="${phase}-${iter}"
fi

stamp="$(date -u +%Y%m%dT%H%M%SZ)"
root_dir="$(cd "$(dirname "$0")/.." && pwd)"
report_dir="$root_dir/reports/${target_ref}-${stamp}"
mkdir -p "$report_dir"

run_scan() {
  name="$1"
  shift

  echo "==> $name"
  "$@" 2>&1 | tee "$report_dir/${name}.log"
  rc=${PIPESTATUS[0]}
  printf '%s=%s\n' "$name" "$rc" >>"$report_dir/status.txt"
  return "$rc"
}

write_summary() {
  cat >"$report_dir/README.txt" <<EOF
phase: $phase
iteration: ${iter:-baseline}
target_reference: $target_ref
created_at_utc: $stamp

files:
- oss.json
- oss.log
- code.json
- code.log
- monitor.log
- status.txt
EOF
}

write_summary

oss_rc=0
code_rc=0
monitor_rc=0

run_scan oss \
  snyk test \
  --debug \
  --log-level=trace \
  --json-file-output="$report_dir/oss.json" || oss_rc=$?

run_scan code \
  snyk code test \
  --debug \
  --log-level=trace \
  --json-file-output="$report_dir/code.json" || code_rc=$?

run_scan monitor \
  snyk monitor \
  --debug \
  --log-level=trace \
  --target-reference="$target_ref" || monitor_rc=$?

echo
echo "Reports written to: $report_dir"
echo "Exit codes: oss=$oss_rc code=$code_rc monitor=$monitor_rc"

if [ "$oss_rc" -gt 1 ] || [ "$code_rc" -gt 1 ] || [ "$monitor_rc" -gt 1 ]; then
  exit 2
fi
