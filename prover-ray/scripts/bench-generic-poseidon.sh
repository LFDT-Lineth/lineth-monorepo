#!/usr/bin/env bash
# Benchmarks generic Poseidon2 / Fiat-Shamir against non-generic baselines
# (crypto/genericbench), for KoalaBear and MamaBear.
#
# Usage: scripts/bench-generic-poseidon.sh [small|medium|full] [outdir]
#
#   small  quick local run (default)
#   medium wider sizes, a few minutes
#   full   large sizes and repetitions, meant for a dedicated machine
#
# Writes <outdir>/raw.txt (go test -bench output, with CPU info) and, when
# benchstat is installed, <outdir>/summary.txt pivoted per implementation.
# Install benchstat with: go install golang.org/x/perf/cmd/benchstat@latest
set -euo pipefail

SCALE="${1:-small}"
OUT="${2:-bench-generic-poseidon-$SCALE-$(date +%Y%m%d-%H%M%S)}"
cd "$(dirname "$0")/.."

case "$SCALE" in
small) COUNT=3 BENCHTIME=200ms TIMEOUT=30m ;;
medium) COUNT=6 BENCHTIME=200ms TIMEOUT=1h ;;
full) COUNT=10 BENCHTIME=2s TIMEOUT=12h ;;
*)
	echo "unknown scale: $SCALE (want small, medium or full)" >&2
	exit 1
	;;
esac

mkdir -p "$OUT"
{
	echo "# scale: $SCALE"
	echo "# commit: $(git rev-parse HEAD)"
	echo "# go: $(go version)"
	echo "# host: $(uname -a)"
	if [[ -r /proc/cpuinfo ]]; then
		echo "# cpu: $(grep -m1 'model name' /proc/cpuinfo | cut -d: -f2-)"
		echo "# avx512f: $(grep -qw avx512f /proc/cpuinfo && echo yes || echo no)"
	else
		echo "# cpu: $(sysctl -n machdep.cpu.brand_string 2>/dev/null || echo unknown)"
	fi
	echo "# nproc: $(getconf _NPROCESSORS_ONLN)"
} >"$OUT/raw.txt"

# The agreement test runs first: a failure means the implementations do not
# compute the same thing, so the timings would be meaningless.
GENERIC_BENCH_SCALE="$SCALE" go test ./crypto/genericbench/ \
	-run TestAgreement -bench . -benchmem \
	-count "$COUNT" -benchtime "$BENCHTIME" -timeout "$TIMEOUT" |
	tee -a "$OUT/raw.txt"

if command -v benchstat >/dev/null; then
	benchstat -col /impl "$OUT/raw.txt" | tee "$OUT/summary.txt"
else
	echo "benchstat not found; raw results in $OUT/raw.txt" >&2
fi
