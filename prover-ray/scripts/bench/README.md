# Aggregation bench (local test branch only)

Runs the real rollup-aggregation guest with no coordinator, on the guest's
committed sample request (`riscv-guests/rollup_aggregation/test/testdata/sample.request.ssz`,
real devmode rollup proofs), and checks the result against its committed output
(`sample.output.bin`). Run it on the bench machine, from the repository root.

Needs Zig (`riscv-guests/.zigversion`) and Go on `PATH`, and the spec's Python
dependencies (`pip install -r rollup_spec/requirements.txt`, in a venv).

```
B=/tmp/aggregation-bench; mkdir -p $B/elf $B/guests $B/publicInputs $B/guestOutput
IN=riscv-guests/rollup_aggregation/test/testdata/sample.request.ssz
OUT=riscv-guests/rollup_aggregation/test/testdata/sample.output.bin

# 1. Aggregation ELF, built without Docker, staged under its Program ID.
make -C riscv-guests/rollup_aggregation compile
make -C riscv-guests release-asset GUEST=rollup-aggregation RELEASE_VERSION=0.0.0-dev RELEASE_DIR=$B/elf
id=$(GUEST=rollup-aggregation prover-ray/scripts/stage-guest.sh $B/guests $B/elf)

# 2. Backend: the guest on the sample request, in dev-zkvm and dev-partial.
(cd prover-ray && REAL_GUEST_TESTS=1 go test ./backend -run TestAggregation_Integration -v -timeout 0)

# 3. Request level, with the embedded proofs as publicInputs, then as guestOutput:
#    prover-ray builds the same guest input from the request, proves it, and the
#    response carries the guest output and a BN254 PLONK proof.
(cd prover-ray && go build -o $B/prover ./cmd/prover)
for f in publicInputs guestOutput; do
  req=$(python3 prover-ray/scripts/bench/aggregation_request.py request $IN "$id" $f $B/$f)
  (cd prover-ray && AGGREGATION_BENCH_REQUEST=$req AGGREGATION_BENCH_INPUT=$PWD/../$IN \
    go test ./backend/jobadapter/ -run TestAggregationBench_Input -count=1 -v)
  time $B/prover prove --config prover-ray/scripts/bench/config-aggregation.toml --in "$req" --out $B/$f/response.json
  python3 prover-ray/scripts/bench/aggregation_request.py check $B/$f/response.json $OUT
done
```

Step 3 runs dev-zkvm. For dev-partial, set `aggregation.prover_mode = "dev-partial"`
in `config-aggregation.toml` and run step 3 again.
