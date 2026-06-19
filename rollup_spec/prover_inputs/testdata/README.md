# V1 prover I/O test fixtures

Fully-valid `getZkL2ExecutionProofV1`, `getZkRollupProofV1`, and
`getZkRollupAggregationProofV1` request/response payloads (real hex, no ellipses)
used by `rollup_spec/proof_io_v1_test.py`.

These fixtures are the **language-neutral contract** for the prover wire format:
each layer's request/response pair is mutually consistent, so the codec
round-trip (`decode_*` then `encode_*`) holds, and any implementation (Go prover,
Kotlin coordinator, …) can load them as golden vectors and assert its own
serializer round-trips them byte-for-byte. There is no separate JSON Schema; the
codec in `rollup_spec/proof_io_v1.py` is the wire authority and the guest
dataclasses are the logical model.

## Fields ↔ guest dataclasses

Each fixture's fields correspond to the input/output class of the entry function
of the matching guest program (the codec in `rollup_spec/proof_io_v1.py` converts
between them). A request maps to the entry function's input dataclass; a response
maps to its output.

| Fixture | Guest dataclass | Defined in | Guest entry function |
|---|---|---|---|
| `getZkL2ExecutionProofV1.request.json` | `L2ExecutionProofPrivateInput` | `l2_execution.py` | `run_l2_execution_guest` (input) |
| `getZkL2ExecutionProofV1.response.json` | `L2ExecutionProof` | `l2_execution.py` | `run_l2_execution_guest` (output) |
| `getZkRollupProofV1.request.json` | `RollupProofPrivateInput` | `rollup.py` | `run_rollup_guest` (input) |
| `getZkRollupProofV1.response.json` | `RollupProof` | `rollup.py` | `run_rollup_guest` (output) |
| `getZkRollupAggregationProofV1.request.json` | `RollupAggregationProofPrivateInput` | `rollup_aggregation.py` | `run_rollup_aggregation_guest` (input) |
| `getZkRollupAggregationProofV1.response.json` | `FinalizationSubmission` | `l1_rollup.py` | `run_rollup_aggregation_guest` (output) |

**Guest output vs prover output.** A guest emits its public-input tuple plus the
revealed hash preimages (`l2L1Messages`, `txFroms`, `l2L1Roots`,
`filteredAddresses`). The `proof` bytes are attached by the zkVM/prover layer,
not the guest, so they are placeholders (`0x`) in these fixtures; a response
equals the guest output plus `proof`. The aggregation response is a
`FinalizationSubmission`: it additionally carries `l2L1Roots`,
`filteredAddresses`, and `l2MessagingBlocksOffsets` — the preimages the L1
`finalize_rollup` call consumes as calldata — so it is sufficient for L1
finalization.

The JSON field names are not always a 1:1 camel↔snake mapping of the dataclass
fields; the codec owns the renames and type coercion (see `proof_io_v1.py`). A
few request fields are metadata the entry-function input dataclass does not
carry: `proverVersion` and the top-level `blockRange` (the range is implied by
the payloads/blobs) on every request, plus `chainId` on the rollup request (used
for DA sender recovery). Derivable duplication is deliberately kept off the wire:
no top-level `endBlockNumber` (it is in `publicInputs`), no
`shnarfTransition.endShnarf` echo on the rollup request (the guest recomputes it
and returns it in the response PI), and no `chainId` on the aggregation request.

## Running the tests locally

`rollup_spec/proof_io_v1_test.py` imports the guest dataclasses, which pull in the
native dependencies in `rollup_spec/requirements.txt` (`ckzg`, `coincurve` via
`ethereum-execution`, `lz4`). Those have no wheels for the newest Python and are
built from source, so use **Python 3.11 or 3.12** and the Xcode command-line
tools on macOS.

Prerequisites:

- Python 3.11 or 3.12 (the pinned `coincurve`/`ckzg` builds fail on 3.13+).
- A C toolchain for the native builds. On macOS: `xcode-select --install`.

Set up an isolated environment and install the dependencies (run from the repo
root):

```bash
cd rollup_spec
python3.12 -m venv .venv
source .venv/bin/activate
python -m pip install --upgrade pip
python -m pip install -r requirements.txt
python -m pip install pytest          # the test runner itself is not in requirements.txt
```

Run the tests from the **repo root** (so the `rollup_spec` package resolves):

```bash
cd ..                       # back to the repo root
python -m pytest rollup_spec/proof_io_v1_test.py
```

When you are done, `deactivate` the virtualenv.
