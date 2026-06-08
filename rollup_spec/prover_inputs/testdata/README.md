# V1 prover I/O test fixtures

Fully-valid `getZkL2ExecutionProofV1`, `getZkRollupProofV1`, and
`getZkRollupAggregationProofV1` request/response payloads (real hex, no ellipses)
used by `rollup_spec/proof_io_v1_test.py`.

These differ from the illustrative examples in `../` (the parent
`prover_inputs/` directory): those use `0x...` placeholders for documentation and
therefore do **not** validate against the JSON Schemas in `rollup_spec/schemas/`.
The fixtures here do validate, and the request/response pair is mutually
consistent so the codec round-trip (`decode_request`, `encode_response`) can be
asserted byte-for-byte.

## Fields ↔ guest dataclasses

Each fixture's fields correspond to the input/output class of the entry function
of the matching guest program (the codec in `rollup_spec/proof_io_v1.py` converts
between them). A request maps to the entry function's input dataclass; a response
maps to its output dataclass.

| Fixture | Guest dataclass | Defined in | Guest entry function |
|---|---|---|---|
| `getZkL2ExecutionProofV1.request.json` | `L2ExecutionProofPrivateInput` | `l2_execution.py` | `run_l2_execution_guest` (input) |
| `getZkL2ExecutionProofV1.response.json` | `L2ExecutionProof` | `l2_execution.py` | `run_l2_execution_guest` (output) |
| `getZkRollupProofV1.request.json` | `RollupProofPrivateInput` | `rollup.py` | `run_rollup_guest` (input) |
| `getZkRollupProofV1.response.json` | `RollupProof` | `rollup.py` | `run_rollup_guest` (output) |
| `getZkRollupAggregationProofV1.request.json` | `RollupAggregationProofPrivateInput` | `rollup_aggregation.py` | `run_rollup_aggregation_guest` (input) |
| `getZkRollupAggregationProofV1.response.json` | `RollupPublicInput` | `rollup_aggregation.py` | `run_rollup_aggregation_guest` (output) |

Note the JSON field names are not always a 1:1 camel↔snake mapping of the
dataclass fields; the codec owns the renames and type coercion (see
`proof_io_v1.py`). A few request fields are metadata that the entry-function
input dataclass does not carry (e.g. `proverVersion`, the top-level `blockRange`,
the rollup request's `shnarfTransition.endShnarf`, the aggregation request's
`chainId`); the codec ignores them when building the dataclass.

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

`requirements.txt` already includes `jsonschema`, which the schema-conformance
tests need; without it those tests are skipped (via `pytest.importorskip`) rather
than failing.

Run the tests from the **repo root** (so the `rollup_spec` package resolves):

```bash
cd ..                       # back to the repo root
python -m pytest rollup_spec/proof_io_v1_test.py
```

When you are done, `deactivate` the virtualenv.
