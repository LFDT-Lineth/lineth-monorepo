# Rollup bench (local test branch only)

Runs a rollup request through `prover prove` with the real rollup guest, with no
coordinator. The request is written from the rollup guest's fixture
(`sample.request.ssz`), and the response is checked against its output
(`sample.output.bin`). Run it on the bench machine, from the repository root.

Needs Zig (`riscv-guests/.zigversion`) and Go on `PATH`, and the spec's Python
dependencies (`pip install -r rollup_spec/requirements.txt`).

```
B=/tmp/rollup-bench; mkdir -p $B/elf $B/guests

# 1. Rollup ELF, built without Docker, and the fixture.
make -C riscv-guests/rollup compile
make -C riscv-guests/rollup require-input
make -C riscv-guests release-asset GUEST=rollup RELEASE_VERSION=0.0.0-dev RELEASE_DIR=$B/elf
id=$(GUEST=rollup prover-ray/scripts/stage-guest.sh $B/guests $B/elf)

# 2. The request.
req=$(python3 prover-ray/scripts/bench/rollup_request.py request \
  riscv-guests/rollup/zig-out/sample.request.ssz "$id" $B)

# 3. prover-ray builds the same guest input from the request (no guest run).
(cd prover-ray && ROLLUP_BENCH_REQUEST=$req \
  ROLLUP_BENCH_INPUT=$PWD/../riscv-guests/rollup/zig-out/sample.request.ssz \
  go test ./backend/jobadapter/ -run TestRollupBench -v)

# 4. Prove, then check the response.
(cd prover-ray && go build -o $B/prover ./cmd/prover)
time $B/prover prove --config prover-ray/scripts/bench/config-rollup.toml --in "$req" --out $B/response.json
python3 prover-ray/scripts/bench/rollup_request.py check $B/response.json \
  riscv-guests/rollup/zig-out/sample.output.bin
```

dev-zkvm takes about a minute. dev-partial (set it in `config-rollup.toml`)
took 32 minutes and 73 GiB peak memory with 500000 instructions per shard.
