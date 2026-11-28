#!/usr/bin/env bash
# Runs the aggregation bench of README.md from start to end: builds and stages
# the guest, runs the backend test in dev-zkvm and dev-partial, then the request
# level in both embedded formats in dev-zkvm, then the guestOutput request in
# dev-partial. Stops at the first failure.
#
# Usage, from the repository root, with the spec's Python dependencies
# installed:
#
#	prover-ray/scripts/bench/run-aggregation.sh 2>&1 | tee /tmp/aggregation-bench.log
set -euo pipefail

B=/tmp/aggregation-bench
IN=riscv-guests/rollup_aggregation/test/testdata/sample.request.ssz
OUT=riscv-guests/rollup_aggregation/test/testdata/sample.output.bin
CONFIG=prover-ray/scripts/bench/config-aggregation.toml
SCRIPT=prover-ray/scripts/bench/aggregation_request.py

step() { printf '\n=== %s (%s)\n' "$1" "$(date +%T)"; }

python3 -c 'import remerkleable' 2>/dev/null || { echo "the spec's Python dependencies are missing: activate the venv"; exit 1; }
rm -rf "$B"
mkdir -p "$B/elf" "$B/guests" "$B/publicInputs" "$B/guestOutput"

step "1. build and stage the aggregation guest"
make -C riscv-guests/rollup_aggregation compile
make -C riscv-guests release-asset GUEST=rollup-aggregation RELEASE_VERSION=0.0.0-dev RELEASE_DIR="$B/elf"
id=$(GUEST=rollup-aggregation prover-ray/scripts/stage-guest.sh "$B/guests" "$B/elf")
echo "Program ID: $id"

step "2. backend: real guest in dev-zkvm and dev-partial"
(cd prover-ray && REAL_GUEST_TESTS=1 go test ./backend -run TestAggregation_Integration -count=1 -v -timeout 0)

step "3. build the prover"
(cd prover-ray && go build -o "$B/prover" ./cmd/prover)

for form in publicInputs guestOutput; do
	step "request with $form, dev-zkvm"
	req=$(python3 "$SCRIPT" request "$IN" "$id" "$form" "$B/$form")
	echo "request: $req"
	(cd prover-ray && AGGREGATION_BENCH_REQUEST="$req" AGGREGATION_BENCH_INPUT="$PWD/../$IN" \
		go test ./backend/jobadapter/ -run TestAggregationBench_Input -count=1 -v)
	time "$B/prover" prove --config "$CONFIG" --in "$req" --out "$B/$form/response.json"
	python3 "$SCRIPT" check "$B/$form/response.json" "$OUT"
done

step "request with guestOutput, dev-partial"
sed 's/^prover_mode = "dev-zkvm"/prover_mode = "dev-partial"/' "$CONFIG" >"$B/config-dev-partial.toml"
time "$B/prover" prove --config "$B/config-dev-partial.toml" --in "$req" --out "$B/guestOutput/response-dev-partial.json"
python3 "$SCRIPT" check "$B/guestOutput/response-dev-partial.json" "$OUT"

step "all steps passed"
