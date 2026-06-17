# V1 prover I/O test fixtures

Fully-valid `getZkL2ExecutionProofV1`, `getZkRollupProofV1`, and
`getZkRollupAggregationProofV1` request/response payloads (real hex, no ellipses)
used by `rollup_spec/prover_inputs/schemas/fixture_schema_conformance_test.py`.

These differ from the illustrative examples in `../` (the parent
`prover_inputs/` directory): those use `0x...` placeholders for documentation and
therefore do **not** validate against the JSON Schemas in `rollup_spec/prover_inputs/schemas/`.
The fixtures here do validate against those schemas (see the conformance test
below).

## Fields ↔ guest dataclasses

Each fixture's fields correspond to the input/output class of the entry function
of the matching guest program. A request maps to the entry function's input
dataclass; a response maps to its output dataclass.

| Fixture | Guest dataclass | Defined in | Guest entry function |
|---|---|---|---|
| `getZkL2ExecutionProofV1.request.json` | `L2ExecutionProofPrivateInput` | `l2_execution.py` | `run_l2_execution_guest` (input) |
| `getZkL2ExecutionProofV1.response.json` | `L2ExecutionProof` | `l2_execution.py` | `run_l2_execution_guest` (output) |
| `getZkRollupProofV1.request.json` | `RollupProofPrivateInput` | `rollup.py` | `run_rollup_guest` (input) |
| `getZkRollupProofV1.response.json` | `RollupProof` | `rollup.py` | `run_rollup_guest` (output) |
| `getZkRollupAggregationProofV1.request.json` | `RollupAggregationProofPrivateInput` | `rollup_aggregation.py` | `run_rollup_aggregation_guest` (input) |
| `getZkRollupAggregationProofV1.response.json` | `RollupPublicInput` | `rollup_aggregation.py` | `run_rollup_aggregation_guest` (output) |

Note the JSON field names are not always a 1:1 camel↔snake mapping of the
dataclass fields (there are semantic renames and type coercion between the wire
form and the dataclasses). A few request fields are metadata that the
entry-function input dataclass does not carry (e.g. `proverVersion`, the
top-level `blockRange`, the rollup request's `shnarfTransition.endShnarf`, the
aggregation request's `chainId`).

## Running the conformance test locally

`fixture_schema_conformance_test.py` checks that every `fixture/<name>.json`
validates against its matching `<name>.schema.json`, and that each schema is
itself a valid Draft 2020-12 schema. Fixtures are discovered automatically, so
new fixture/schema pairs are covered without editing the test.

The test has **no native dependencies** — only `pytest` and `jsonschema` — and
runs on any modern Python (no 3.11/3.12 constraint, no C toolchain). Run it from
the **repo root** so the `rollup_spec` package resolves:

```bash
python3 -m venv .venv
source .venv/bin/activate
python -m pip install --upgrade pip
python -m pip install pytest jsonschema
python -m pytest rollup_spec/prover_inputs/schemas/fixture_schema_conformance_test.py
```

If `jsonschema` is missing, the validation cases are skipped (via
`pytest.importorskip`) rather than failing.

When you are done, `deactivate` the virtualenv.
