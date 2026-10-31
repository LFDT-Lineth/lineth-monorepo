# Running the l2-execution guest under zkc

How to execute, time, and trace the `l2-execution` guest on real blocks and on the large
(30M–200M gas) execution-spec benchmark fixtures, using the `zkc` that matches this checkout.

Contents: [Prerequisites](#prerequisites) · [Input format](#input-format) ·
[Quick path](#quick-path-make-targets) · [Getting inputs](#getting-inputs) ·
[Custom blocks](#custom-blocks) · [Building guest variants](#building-guest-variants) · [Timing `exec --gogen`](#timing-zkc-exec---gogen) ·
[Counting shards](#counting-shards-zkc-trace) · [Reference results](#reference-results) ·
[Pitfalls](#pitfalls)

Set these once per shell; every command below uses them and none depends on the current directory:

```bash
export REPO=$(git rev-parse --show-toplevel)   # run from anywhere inside the checkout
export WORK=/tmp/l2-bench                      # scratch space for inputs, JSON, logs
```

## Prerequisites

| Tool | Used for | Notes |
|---|---|---|
| Zig `0.16.0` | building the guest ELF and host tools | pinned in `riscv-guests/.zigversion` |
| Go | `go tool zkc`, `go tool elf_to_json` | run **inside `arithmetization/`** — `arithmetization/go.mod` pins the `zkc` version (currently `v1.2.35`; the reference results below were measured on `v1.2.34`), so you get the `zkc` that matches the arithmetization sources |
| Nim, `clang`, `llvm-ar` | building the Constantine staticlib (any build, both backends) | `make -C riscv-guests install-constantine-deps` |
| `jq`, `xxd` | unpacking `statelessInputBytes` from fixture JSON | |
| `gh` or `curl` | downloading benchmark fixtures | `gh` needs no auth for public releases |
| `taskset` (optional) | pinning tracing to 16 CPUs like CI does | |

Never invoke a globally installed `zkc` for comparisons: it may not match `arithmetization/src/main/riscv/main.zkc`.

## Input format

The guest reads the **extended** `L2ExecutionProofPrivateInput` SSZ (schema `0x0002`, first two bytes
`00 02`). It does **not** accept a vanilla EF `StatelessInput` directly.

- Committed samples and the `block_amsterdam_*` blocks are already extended.
- Anything extracted from execution-spec fixtures (`statelessInputBytes`) is vanilla and must be
  wrapped with `l2-execution-wrap`, which fills dummy rollup fields (zero `l2MessageServiceAddress`, so
  the guest suppresses the L1<->L2 bridge reads). See [`test/l2_execution_wrap.zig`](../../test/l2_execution_wrap.zig).
  It exits with code `3` for blocks that carry EIP-7685 execution requests; the guest rejects those by
  policy, so skip them.

Check an input: `head -c 2 input.ssz | xxd -p` → `0002`.

## Quick path: make targets

Fine for one input and the default build (standard Zig keccak, `std` secp256k1 backend):

```bash
cd $REPO/riscv-guests/l2_execution

make exec  INPUT=/abs/path/input.ssz ZKC_EXEC_FLAGS=--gogen                    # fast execution
make exec  INPUT=/abs/path/input.ssz ZKC_EXEC_FLAGS=--gogen SECP256K1_BACKEND=constantine
make trace INPUT=/abs/path/input.ssz ZKC_TRACE_FLAGS="--stats -vv --sharding interpreter:1000000"
```

`INPUT` defaults to `test/testdata/stateless_input.ssz`. These targets rebuild
`zig-out/bin/evm_execution_guest` every time, so they cannot hold two backends side by side — use the
explicit flow below for comparisons. `make exec`/`trace` print the guest result as
`guest_output = 0x0003…` (schema `0003` = accepted).

## Getting inputs

Create the scratch layout:

```bash
mkdir -p $WORK/{blocks,vanilla,wrapped,json,logs}
```

### 1. Committed sample (tiny)

`test/testdata/stateless_input.ssz` — 4.9 KB, about 35 keccak calls. Good for smoke tests only.

### 2. Sample blocks at 175k – 17M gas (PR #4139)

PR [#4139](https://github.com/LFDT-Lineth/lineth-monorepo/pull/4139) adds five extended-format blocks to
`test/testdata/`. Once it is merged they are in this directory; until then take them from the PR ref
without switching branches:

```bash
git fetch origin pull/4139/head:refs/remotes/origin/pr-4139
for b in 175k 51584_500k 52861_2M 52866_17M 786k; do
  git show origin/pr-4139:riscv-guests/l2-execution/test/testdata/block_amsterdam_$b.ssz \
    > $WORK/blocks/block_amsterdam_$b.ssz
done
(cd $WORK/blocks && sha256sum -c - <<'EOF'
95912ab0b63098fe2b22241c6f489b97f70da94589d09fdb3d0af534d363b913  block_amsterdam_175k.ssz
5069faefb0c187f334ec44aaac5c82ca3e83cdc720571f1ca4884a01c723473c  block_amsterdam_51584_500k.ssz
58b284b62771b08f45c22518af248f58daca22b0d702cacb889f4b6ea947127c  block_amsterdam_52861_2M.ssz
a8ef276bb6684920c39d99c6724084b04bbd79a7805daedb6b7539040ad4be0f  block_amsterdam_52866_17M.ssz
4acee6b97065a4c5930963ea198a784f3a54d95ebee5dd006bab65139547753c  block_amsterdam_786k.ssz
EOF
)
```

The number in the name is the gas used. Whether these are mainnet blocks is not recorded in the PR; the
block numbers (51584–52866) look like a devnet/testnet.

### 3. Large execution-spec benchmark fixtures (30M – 200M gas)

Source: GitHub release `tests-zkevm-benchmark@v21.0.5` of `ethereum/execution-specs`. The version
matches the `tests-zkevm@v21.0.5` fixtures pinned in `build.zig.zon`, so the stateless-input layout is
the one the pinned Zesu decodes. One tarball per gas budget:

| Asset | Size |
|---|---|
| `fixtures_zkevm-benchmark_0030M.tar.gz` | 156 MB |
| `fixtures_zkevm-benchmark_0060M.tar.gz` | 288 MB |
| `fixtures_zkevm-benchmark_0100M.tar.gz` | 464 MB |
| `fixtures_zkevm-benchmark_0150M.tar.gz` | 684 MB |
| `fixtures_zkevm-benchmark_0200M.tar.gz` | 903 MB |

```bash
cd $WORK
gh release download 'tests-zkevm-benchmark@v21.0.5' -R ethereum/execution-specs \
  -p 'fixtures_zkevm-benchmark_0030M.tar.gz'
# without gh:
# curl -LO 'https://github.com/ethereum/execution-specs/releases/download/tests-zkevm-benchmark%40v21.0.5/fixtures_zkevm-benchmark_0030M.tar.gz'

echo '157527b8ca656f051af541767c9a57a56d867f90dd4f2435d02a9eb592476bef  fixtures_zkevm-benchmark_0030M.tar.gz' | sha256sum -c -
mkdir -p fixtures && tar -xzf fixtures_zkevm-benchmark_0030M.tar.gz -C fixtures
```

Layout: `fixtures/fixtures/blockchain_tests/for_amsterdam_at_0030M/compute/<group>/<…>/<test>.json` — 121
files, about 1200 cases at 30M. Every case is an EEST `blockchain_test` whose `blocks[]` entries carry
`statelessInputBytes` (hex, vanilla SSZ) and the block header (`blockHeader.gasUsed`).

List the cases of a file with their block count and per-block gas used:

```bash
F=$WORK/fixtures/fixtures/blockchain_tests/for_amsterdam_at_0030M/compute
jq -r 'to_entries[] | [(.key|sub("^.*::";"")), (.value.blocks|length),
       ([.value.blocks[].blockHeader.gasUsed]|join(","))] | @tsv' \
  $F/instruction/storage/storage_access_warm.json
```

Note that `storage_access_warm` has **two blocks per case**: a ~0.3M-gas setup block, then the 30M-gas
benchmark block. Pick the block you mean.

Unpack one case/block and wrap it for the guest:

```bash
# extract <fixture.json> <case index> <block index> <out.ssz>
extract() {
  jq -r --argjson c "$2" --argjson b "$3" 'to_entries[$c].value.blocks[$b].statelessInputBytes' "$1" \
    | sed 's/^0x//' | xxd -r -p > "$4"
}

# build the wrap tool once (see "Building guest variants" for the guests)
(cd $REPO/riscv-guests/l2_execution && zig build --prefix $WORK/host -Dhost-tools=true)

extract $F/instruction/arithmetic/arithmetic.json 0 0 $WORK/vanilla/arithmetic_0_30M.ssz
$WORK/host/bin/l2-execution-wrap $WORK/vanilla/arithmetic_0_30M.ssz $WORK/wrapped/arithmetic_0_30M.ssz
```

The set used for the reference results below (first case of each file; `storage_access_warm` uses
block 1):

| Name | Fixture (under `…/compute/`) |
|---|---|
| `arithmetic` | `instruction/arithmetic/arithmetic.json` |
| `mod` | `instruction/arithmetic/mod.json` |
| `jump_benchmark` | `instruction/control_flow/jump_benchmark.json` |
| `keccak` | `instruction/keccak/keccak.json` |
| `mcopy` | `instruction/memory/mcopy.json` |
| `storage_access_warm` | `instruction/storage/storage_access_warm.json` (block 1) |
| `ether_transfers` | `scenario/transaction_types/ether_transfers.json` |

`ether_transfers` is the only one with many transactions (2500 value transfers in case 0, each needing
sender recovery); it is the case that separates the two secp256k1 backends.

### 4. Other sources (not exercised while writing this guide)

- EF correctness fixtures (`tests-zkevm@v21.0.5`, 6 GB, lazy Zig dependency): `make -C riscv-guests/l2_execution
  prep-execution-specs-json-fixtures` exposes them under `/tmp/execution-specs-json-fixtures/fixtures`;
  `make reference-test-zkc` runs the corpus through the ELF under zkc. These check validity verdicts and
  are mostly tiny blocks.
- Live-network stateless inputs: `eth-act/zkevm-benchmark-workload`'s `witness-generator-spec-cli`
  produces and publishes EEST `blockchain_test_engine` fixtures (a public Sepolia dataset is
  described in its `docs/stateless-input-publication.md`). Same unpack-and-wrap steps apply.

## Custom blocks

To benchmark blocks that nobody published (N transfers, a given precompile, "prover killers"), generate
them with EEST itself. Its in-repo EELS `t8n` builds the execution witness itself
(`packages/testing/.../evm_tools/t8n/result.py`, `build_execution_witness`), and `fill` serialises the
result into the same `statelessInputBytes` the release fixtures carry. No other route was tried: the
geth/reth/Besu adapters in EEST's `client_clis/` and Zesu's own `t8n` source contain no witness
handling, but I did not run those `evm t8n` binaries, so treat them as unverified. Filling is fast: all
7 cases below at 30M gas took 13 s with 4 workers (EELS in pure Python).

```bash
# one-time setup; uv is installed into a throwaway venv so nothing global changes
mkdir -p $WORK/eest && cd $WORK/eest
git clone --depth 1 --branch 'tests-zkevm-benchmark@v21.0.5' https://github.com/ethereum/execution-specs.git
python3 -m venv uvenv && ./uvenv/bin/pip install -q uv
cd execution-specs && ../uvenv/bin/uv sync --all-extras
```

Pin the same tag as the fixtures (`v21.0.5`): the SSZ layout must match the Zesu decoder pinned in
`build.zig.zon`.

Write a test under `tests/benchmark/compute/` (it must live there to get the `benchmark_test`,
`pre` and `gas_benchmark_value` fixtures). Two examples that were run for this guide:

```python
# tests/benchmark/compute/custom/test_zkc_custom.py   (+ an empty __init__.py next to it)
import pytest
from execution_testing import (Alloc, BenchmarkTestFiller, Block, Fork,
                               JumpLoopGenerator, Op, RecipientType, Transaction)

@pytest.mark.parametrize("tx_count", [250, 1000])
@pytest.mark.parametrize("distinct_senders", [True, False], ids=["distinct", "single"])
def test_fixed_count_transfers(benchmark_test: BenchmarkTestFiller, pre: Alloc, fork: Fork,
                               gas_benchmark_value: int, tx_count: int, distinct_senders: bool) -> None:
    """tx_count plain transfers to an existing account: isolates sender recovery."""
    cost = (fork.transaction_intrinsic_cost_calculator()(
                sends_value=True, recipient_type=RecipientType.CONTRACT)
            + fork.transaction_top_frame_execution_gas(
                sends_value=True, recipient_type=RecipientType.CONTRACT))
    if tx_count * cost > gas_benchmark_value:
        pytest.skip("tx_count does not fit the gas budget")
    receiver, shared = pre.fund_eoa(1), pre.fund_eoa()
    txs = [Transaction(to=receiver, value=1, gas_limit=cost,
                       sender=pre.fund_eoa() if distinct_senders else shared)
           for _ in range(tx_count)]
    benchmark_test(pre=pre, blocks=[Block(txs=txs)], expected_benchmark_gas_used=tx_count * cost)

@pytest.mark.parametrize("hash_bytes", [1024, 32 * 1024, 128 * 1024])
def test_keccak_hash_bomb(benchmark_test: BenchmarkTestFiller, pre: Alloc, hash_bytes: int) -> None:
    """Loop KECCAK256 over hash_bytes of memory until the gas budget is spent."""
    benchmark_test(pre=pre, code_generator=JumpLoopGenerator(
        setup=Op.MSTORE(hash_bytes - 32, 1), attack_block=Op.POP(Op.SHA3(0, hash_bytes))))
```

```bash
cd $WORK/eest/execution-specs
../uvenv/bin/uv run fill --gas-benchmark-values 30 --fork Amsterdam -m "not slow and primary_format" \
  --skip-index --clean --output=$WORK/eest/out -n 4 tests/benchmark/compute/custom
# -> $WORK/eest/out/blockchain_tests/for_amsterdam_at_0030M/compute/custom/zkc_custom/*.json
```

`--gas-benchmark-values N` is in millions of gas and picks the budget (`1` = 1M); the transfer cases skip
themselves at 1M. From here the flow is the same as for release fixtures: unpack
`blocks[-1].statelessInputBytes` with `jq`/`extract` (the last block is the benchmark block; helper
blocks come first), wrap with `l2-execution-wrap`, then time with `bench_exec.sh`.

Existing upstream tests already cover most "prover killer" candidates; reuse them via `fill -k <name>`
instead of rewriting them. Directories under `tests/benchmark/compute/`:
`precompile/` (`ecrecover`, `modexp`, `alt_bn128` incl. pairings, `bls12_381` incl. G1/G2 MSM and
pairing, `blake2f`, `p256verify`, `point_evaluation`, `sha256`, `ripemd160`, `identity`; each also has an
`_uncachable` variant), `instruction/` (`keccak`, `memory`/`mcopy`, `storage`, `system` with
CREATE/CALL/SELFDESTRUCT and `contract_calling_many_addresses`, `account_query` incl. cold
`EXTCODESIZE`/`EXTCODECOPY`, `log`, ...), and `scenario/` (`transaction_types` incl.
`block_full_data`, `block_full_access_list_and_data`, `auth_transaction`, `contract_creation`;
`unchunkified_bytecode`; `jumpdest_analysis`).

Measured with these two tests (30M gas, `exec --gogen`, mean of 2, same machine as the reference
results, `-Dkeccak-accel=false`; every run accepted):

| Block | Txs | std (s) | constantine (s) | Per tx, std / constantine |
|---|---|---|---|---|
| 250 transfers, single sender | 250 | 6.07 | 12.57 | 24 ms / 50 ms |
| 250 transfers, distinct senders | 250 | 6.24 | 12.74 | 25 ms / 51 ms |
| 1000 transfers, single sender | 1000 | 24.26 | 49.31 | 24 ms / 49 ms |
| 1000 transfers, distinct senders | 1000 | 23.22 | 50.54 | 23 ms / 51 ms |
| keccak, 1 KiB per hash | 2 | 39.2 | 40.0 | |
| keccak, 32 KiB per hash | 2 | 42.5 | 43.2 | |
| keccak, 128 KiB per hash | 2 | 43.2 | 44.1 | |

Sender recovery costs the same per transaction whether senders repeat or not (every transaction is
verified separately), and cost grows linearly with the transaction count. The keccak loops are 23–26×
slower than the release's `keccak_0_30M` (1.7 s) at identical gas.

## Building guest variants

Build each variant into its own prefix so ELFs coexist. Two independent options matter:

| Option | Values | Default |
|---|---|---|
| `-Dsecp256k1-backend` | `std`, `constantine` | `std` |
| `-Dkeccak-accel` | `true` (custom opcode), `false` (Zig keccak) | `false` |

State both explicitly; the benchmarks below use `-Dkeccak-accel=false`, as CI does for the sample-block
traces.

```bash
cd $REPO/riscv-guests/l2_execution
for v in std constantine; do
  zig build --prefix $WORK/guest-$v -Dsecp256k1-backend=$v -Dkeccak-accel=false
done
# -> $WORK/guest-{std,constantine}/bin/evm_execution_guest
```

Convert an ELF plus an input into the JSON that zkc loads (`0x08800000` is `IN_ORIGIN` from
`riscv-guests/build_common/Makefile`; it must match the linker script the ELF was built with):

```bash
cd $REPO/arithmetization
go tool elf_to_json $WORK/guest-std/bin/evm_execution_guest @$WORK/blocks/block_amsterdam_786k.ssz \
  0x08800000 > $WORK/json/block_amsterdam_786k.std.json     # ~10 MB per JSON
```

Use absolute paths: the command runs from `arithmetization/`.

## Timing `zkc exec --gogen`

```bash
cd $REPO/arithmetization
/usr/bin/time -f "wall=%es maxrss=%MkB" \
  go tool zkc exec --gogen $WORK/json/block_amsterdam_786k.std.json src/main/riscv/main.zkc
```

`--gogen` runs the generated-Go machine; there is no code-generation or compile step in the measured
time. Success prints `guest_output = 0x0003…`; any other schema or a non-zero exit means the guest
rejected the block and the time is meaningless.

Sequential harness (std vs constantine, alternating, results appended to `$WORK/results.tsv`):

```bash
cat > $WORK/bench_exec.sh <<'EOF'
#!/usr/bin/env bash
# usage: bench_exec.sh REPS input.ssz [input.ssz ...]   (extended-schema inputs)
set -eu
WORK=${WORK:-/tmp/l2-bench}
ARITH=${ARITH:-${REPO:?set REPO to the repository root}/arithmetization}
REPS=$1; shift
inputs=$(realpath -- "$@")            # resolve before chdir
mkdir -p "$WORK/json" "$WORK/logs"
cd "$ARITH"
for f in $inputs; do
  base=$(basename "$f" .ssz)
  for v in std constantine; do
    j=$WORK/json/$base.$v.json
    [ -s "$j" ] || go tool elf_to_json "$WORK/guest-$v/bin/evm_execution_guest" "@$f" 0x08800000 > "$j" \
      || { rm -f "$j"; echo "elf_to_json failed: $f" >&2; exit 1; }
  done
  for r in $(seq 1 "$REPS"); do
    for v in std constantine; do
      log=$WORK/logs/$base.$v.$r.log
      rc=0
      /usr/bin/time -f "%e %M" -o "$log.time" \
        go tool zkc exec --gogen "$WORK/json/$base.$v.json" src/main/riscv/main.zkc > "$log" 2>&1 || rc=$?
      schema=$(grep -m1 -o 'guest_output = 0x[0-9a-f]\{4\}' "$log" | tail -c 5)
      read -r wall rss < "$log.time"
      printf '%s\t%s\t%s\t%s\t%s\trc=%s schema=%s\n' "$base" "$v" "$r" "$wall" "$rss" "$rc" "${schema:-NONE}" \
        | tee -a "$WORK/results.tsv"
    done
  done
done
EOF
chmod +x $WORK/bench_exec.sh

$WORK/bench_exec.sh 3 $WORK/blocks/*.ssz $WORK/wrapped/*.ssz      # 3 reps; use 1 for ether_transfers
```

Methodology that kept run-to-run spread within ~3.5% (after the first run):

- Run sequentially; nothing else CPU-heavy on the machine.
- Discard or ignore the first run of each input (cold file cache for the ~10 MB JSON; observed 0.81 s
  vs 0.54–0.56 s steady on the 175k block). Report the median of ≥3.
- Alternate the two backends inside each repetition so thermal/cache drift hits both equally.

## Counting shards (`zkc trace`)

```bash
cd $REPO/arithmetization
GOMEMLIMIT=64GiB /usr/bin/time -f "wall=%es maxrss=%MkB" taskset -c 0-15 \
  go tool zkc trace --stats -vv --sharding interpreter:1000000 \
  $WORK/json/block_amsterdam_786k.std.json src/main/riscv/main.zkc > $WORK/logs/trace_786k.std.log 2>&1

grep 'Machine execution' $WORK/logs/trace_786k.std.log        # "(… steps, N checkpoints)"
grep -o 'SHARD [0-9]*' $WORK/logs/trace_786k.std.log | sort -t' ' -k2 -n | tail -1   # highest id
```

- Shard count = `N checkpoints` on the `Machine execution` line = highest `SHARD` id + 1.
- The count scales inversely with the `--sharding interpreter:<size>` argument. CI uses `500000`;
  `1000000` halves the counts. Always quote the value when comparing numbers.
- `-vv` adds per-shard debug lines to the log (0.5–1 MB for the 786k block); `--stats` appends the
  per-module table.
- Resources: tens of GB. 786k-gas block peaked at 42–63 GB RSS (table below); the reference machine
  had 91 GB. `GOMEMLIMIT` is a soft guard; leave headroom for the OS. Tracing the 17M-gas block was
  not tried.
- `make trace` accepts the same flags through `ZKC_TRACE_FLAGS` (the CI step does exactly this).

## Reference results

Measured on 2026-10-27: 16 cores, 91 GB RAM, Linux, repo at `c28dfe0cea` (+ Zesu `a72dec7`,
`zkc v1.2.34`), `-Dkeccak-accel=false`. `exec --gogen` wall time in seconds, median of 3 (1 for
`ether_transfers`); every run was accepted (`guest_output` schema `0003`). RSS was ≈84 000 kB for all but
`ether_transfers` (≈113 000 kB).

| Input | Gas | std | constantine | constantine / std |
|---|---|---|---|---|
| `block_amsterdam_175k` | 0.175M | 0.56 | 0.68 | 1.21 |
| `block_amsterdam_786k` | 0.786M | 0.68 | 0.96 | 1.41 |
| `block_amsterdam_51584_500k` | 0.5M | 0.82 | 1.33 | 1.62 |
| `block_amsterdam_52861_2M` | 2M | 1.37 | 2.29 | 1.67 |
| `block_amsterdam_52866_17M` | 17M | 6.40 | 11.78 | 1.84 |
| `arithmetic_0_30M` | 30M | 6.58 | 6.71 | 1.02 |
| `mod_0_30M` | 30M | 18.10 | 18.18 | 1.00 |
| `jump_benchmark_0_30M` | 30M | 2.41 | 2.48 | 1.03 |
| `keccak_0_30M` | 30M | 1.67 | 1.72 | 1.03 |
| `mcopy_0_30M` | 30M | 4.32 | 4.26 | 0.99 |
| `storage_access_warm_0_30M` (block 1) | 30M | 1.93 | 1.98 | 1.03 |
| `storage_access_warm_0_30M` (block 0) | 0.33M | 0.40 | 0.41 | 1.03 |
| `ether_transfers_0_30M` | 30M | 55.80 | 122.55 | 2.20 |

Reading: the backends are indistinguishable on these opcode-bound cases and `std` wins by 1.2–2.2× as
the share of transactions (sender recovery) grows. Per transaction: about 22–24 ms (`std`) vs
49–50 ms (`constantine`) on a plain value transfer.

Shards for `block_amsterdam_786k` (steps are independent of the sharding size):

| `--sharding` | std | constantine | Steps std / constantine | Trace wall std / constantine | Peak RSS std / constantine |
|---|---|---|---|---|---|
| `interpreter:500000` | 120 | 233 | 1 969 077 299 / 3 762 732 348 | 207 s / 389 s | 42.3 / 59.3 GB |
| `interpreter:1000000` | 60 | 117 | same | 189 s / 352 s | 56.8 / 62.8 GB |

Absolute times depend on the machine; compare ratios and shard counts, not seconds, across machines.

## Pitfalls

- **Wrong input schema.** A vanilla `StatelessInput` fed to the guest is rejected: `zkc` reports
  `machine panic: EXIT CODE = 2` / `execution rejected` and exits with status 4 (observed). Wrap it first.
- **Relative paths.** `go tool …` must run from `arithmetization/`; pass absolute paths for ELF, input,
  and JSON.
- **`storage_access_warm` has two blocks per case.** Block 0 is a setup block (~0.33M gas, ~0.4 s);
  block 1 is the 30M-gas one (~1.9 s).
- **Case 0 is the lightest parametrization, not a stress test.** Every file holds many cases (e.g.
  `keccak.json` has 18) and "same 30M gas" does not mean "same prover work". Case 0 of `keccak.json`
  is `SHA3(0, CALLDATASIZE)` with empty calldata, i.e. hashing 0 bytes (1.7 s), whereas a custom loop
  hashing 1–128 KB per call took about 40 s at the same 30M gas (measured; see
  [Custom blocks](#custom-blocks)). Screen all cases before drawing conclusions about an opcode.
- **Stale JSON.** `bench_exec.sh` reuses `$WORK/json/*.json` when non-empty. Delete them after
  rebuilding a guest or changing `IN_ORIGIN`.
- **`make exec`/`trace` overwrite `zig-out/`.** They always rebuild the default variant (or the one
  given by `SECP256K1_BACKEND` / `KECCAK_ACCEL`) over the same ELF path.
- **Keccak default.** `build.zig` defaults to `-Dkeccak-accel=false` (standard Zig keccak); the prose
  in `README.md` still describes the accelerated wrapper as default. Pass the flag explicitly when
  comparing numbers.
- **Memory.** Tracing is tens of GB; running two traces concurrently on a 91 GB machine is not safe.
