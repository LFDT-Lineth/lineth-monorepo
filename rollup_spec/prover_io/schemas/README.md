# V1 prover I/O JSON Schemas

Draft 2020-12 JSON Schemas for the `getZkL2ExecutionProofV1`,
`getZkRollupProofV1`, and `getZkRollupAggregationProofV1` request/response
payloads — the versioned, language-neutral contract for the Coordinator↔Prover
wire format. `../proof_io_v1.py` is the codec that converts schema-valid JSON
to/from the guest dataclasses; the golden-vector fixtures under `../testdata/`
validate against these schemas (see the conformance test below).

## Fields ↔ guest dataclasses

Each schema's fields correspond to the input/output class of the entry function
of the matching guest program. A request maps to the entry function's input
dataclass; a response maps to its output dataclass.

| Schema | Guest dataclass | Defined in | Guest entry function |
|---|---|---|---|
| `getZkL2ExecutionProofV1.request.schema.json` | `L2ExecutionProofPrivateInput` | `l2_execution.py` | `run_l2_execution_guest` (input) |
| `getZkL2ExecutionProofV1.response.schema.json` | `L2ExecutionProof` | `l2_execution.py` | `run_l2_execution_guest` (output) |
| `getZkRollupProofV1.request.schema.json` | `RollupProofPrivateInput` | `rollup.py` | `run_rollup_guest` (input) |
| `getZkRollupProofV1.response.schema.json` | `RollupProof` | `rollup.py` | `run_rollup_guest` (output) |
| `getZkRollupAggregationProofV1.request.schema.json` | `RollupAggregationProofPrivateInput` | `rollup_aggregation.py` | `run_rollup_aggregation_guest` (input) |
| `getZkRollupAggregationProofV1.response.schema.json` | `FinalizationSubmission` | `l1_rollup.py` | `run_rollup_aggregation_guest` (output) |

Each request is a `{guestProgramId, proofRequest}` envelope. The JSON field names
are not always a 1:1 camel↔snake mapping of the dataclass fields: the codec owns
the renames and type coercion, and a few fields are metadata the guest input
dataclass does not carry (`guestProgramId` on requests, `proverVersion` on
responses, the rollup request's `chainId`). For l2-execution, the request's
`statelessInput` object is SSZ-encoded into the guest's input bytes by the codec
(see `../README.md` and `stateless_input.py`).

## Running the conformance test locally

`fixture_schema_conformance_test.py` checks that every golden-vector fixture
under `../testdata/` validates against its matching `<name>.schema.json`, and
that each schema is itself a valid Draft 2020-12 schema. Fixtures are discovered
automatically, so new fixture/schema pairs are covered without editing the test.

The test has **no native dependencies** — only `pytest` and `jsonschema` — and
runs on any modern Python (no 3.11/3.12 constraint, no C toolchain). Run it from
the **repo root** so the `rollup_spec` package resolves:

```bash
python3 -m venv .venv
source .venv/bin/activate
python -m pip install --upgrade pip
python -m pip install pytest jsonschema
python -m pytest rollup_spec/prover_io/schemas/fixture_schema_conformance_test.py
```

If `jsonschema` is missing, the validation cases are skipped (via
`pytest.importorskip`) rather than failing.

When you are done, `deactivate` the virtualenv.
