"""
Standalone conformance test: every golden-vector fixture under
`rollup_spec/prover_io/testdata/` (both requests and responses) must validate
against its corresponding JSON Schema under `rollup_spec/prover_io/schemas/`.

This test does NOT import the guest dataclasses (only the lightweight,
dependency-free `rollup_spec` package root, to locate the data), so it has no
native dependencies (`ckzg`/`coincurve`/`zstandard`) — only `jsonschema`. It runs on
any Python and is the cheapest way to catch a fixture drifting from its schema.

Fixture <-> schema pairing is by filename convention:

    prover_io/testdata/[<startBlock>-<endBlock>-]<name>.json   <->   prover_io/schemas/<name>.schema.json

A fixture may be prefixed with its block range (e.g. `10-11-` for a sample
covering blocks 10-11), distinguishing multiple samples for the same guest
program; the schema itself is not per-sample, so the prefix is stripped before
lookup.

Fixtures are discovered automatically, so a new fixture/schema pair is covered
without editing this file.

Run from the rollup_spec/ directory:  python -m pytest
"""

import json
import re
from dataclasses import dataclass
from pathlib import Path

import pytest

import rollup_spec

_PROVER_IO_DIR = Path(rollup_spec.__file__).resolve().parent / "prover_io"
_SCHEMA_DIR = _PROVER_IO_DIR / "schemas"
_FIXTURE_DIR = _PROVER_IO_DIR / "testdata"

_BLOCK_RANGE_PREFIX = re.compile(r"^\d+-\d+-")


def _schema_path_for(fixture_path: Path) -> Path:
    """testdata/[<startBlock>-<endBlock>-]<name>.json -> schemas/<name>.schema.json."""
    name = _BLOCK_RANGE_PREFIX.sub("", fixture_path.name)
    return _SCHEMA_DIR / f"{name[: -len('.json')]}.schema.json"


def _fixture_files() -> list[Path]:
    return sorted(_FIXTURE_DIR.glob("*.json"))


def test_fixture_directory_is_not_empty() -> None:
    # Guards against the glob silently matching nothing (e.g. a future move),
    # which would make every parametrized test vacuously "pass" by not running.
    assert _fixture_files(), f"no *.json fixtures found under {_FIXTURE_DIR}"


@pytest.mark.parametrize("fixture_path", _fixture_files(), ids=lambda p: p.name)
def test_fixture_conforms_to_schema(fixture_path: Path) -> None:
    jsonschema = pytest.importorskip("jsonschema")

    schema_path = _schema_path_for(fixture_path)
    assert schema_path.is_file(), (
        f"no schema for fixture {fixture_path.name}; expected {schema_path.name} "
        f"in {_SCHEMA_DIR}"
    )

    schema = json.loads(schema_path.read_text())
    fixture = json.loads(fixture_path.read_text())

    # check_schema first so a malformed schema fails clearly (not as a confusing
    # instance-validation error).
    jsonschema.Draft202012Validator.check_schema(schema)
    jsonschema.Draft202012Validator(schema).validate(fixture)


@pytest.mark.parametrize(
    "schema_path", sorted(_SCHEMA_DIR.glob("*.schema.json")), ids=lambda p: p.name
)
def test_schema_is_valid_draft_2020_12(schema_path: Path) -> None:
    jsonschema = pytest.importorskip("jsonschema")
    schema = json.loads(schema_path.read_text())
    jsonschema.Draft202012Validator.check_schema(schema)


@pytest.mark.parametrize(
    ("fixture_name", "public_inputs"),
    [
        ("10-18-getZkRollupAggregationProofV1.response.json", lambda fixture: [fixture["publicInputs"]]),
    ],
)
def test_l2_l1_tree_depth_schema_accepts_other_guest_depths(fixture_name, public_inputs) -> None:
    jsonschema = pytest.importorskip("jsonschema")
    fixture_path = _FIXTURE_DIR / fixture_name
    schema = json.loads(_schema_path_for(fixture_path).read_text())
    fixture = json.loads(fixture_path.read_text())
    validator = jsonschema.Draft202012Validator(schema)

    for inputs in public_inputs(fixture):
        inputs["l2L1TreeDepth"] = 4
    validator.validate(fixture)

    for invalid_depth in (0, -1, 1.5, 2**64):
        for inputs in public_inputs(fixture):
            inputs["l2L1TreeDepth"] = invalid_depth
        assert not validator.is_valid(fixture), f"depth {invalid_depth} must be rejected"


@pytest.fixture
def execution_request_and_validator():
    jsonschema = pytest.importorskip("jsonschema")
    schema = json.loads(
        (_SCHEMA_DIR / "getZkL2ExecutionProofV1.request.schema.json").read_text()
    )
    request = json.loads(
        (_FIXTURE_DIR / "10-11-getZkL2ExecutionProofV1.request.json").read_text()
    )
    return request, jsonschema.Draft202012Validator(schema)


def _execution_payloads(request: dict) -> list[dict]:
    return [
        payload["statelessInput"]["newPayloadRequest"]["executionPayload"]
        for payload in request["proofRequest"]["payloads"]
    ]


def test_pre_amsterdam_payloads_can_omit_slot(execution_request_and_validator) -> None:
    request, validator = execution_request_and_validator
    request["proofRequest"]["chainConfig"]["forkName"] = "Prague"
    for payload in _execution_payloads(request):
        del payload["slotNumber"]
    validator.validate(request)


def test_slot_zero_is_valid(execution_request_and_validator) -> None:
    request, validator = execution_request_and_validator
    _execution_payloads(request)[0]["slotNumber"] = 0
    validator.validate(request)


@pytest.mark.parametrize("slot", [-1, "10"])
def test_slot_requires_unsigned_integer(execution_request_and_validator, slot) -> None:
    request, validator = execution_request_and_validator
    _execution_payloads(request)[0]["slotNumber"] = slot
    errors = list(validator.iter_errors(request))
    assert any(list(error.path)[-1:] == ["slotNumber"] for error in errors)


@pytest.fixture
def rollup_request_and_validator():
    jsonschema = pytest.importorskip("jsonschema")
    schema = json.loads((_SCHEMA_DIR / "getZkRollupProofV1.request.schema.json").read_text())
    request = json.loads((_FIXTURE_DIR / "10-14-getZkRollupProofV1.request.json").read_text())
    return request, jsonschema.Draft202012Validator(schema)


KZG_FIELDS = ["kzgCommitment", "kzgProof"]


@pytest.mark.parametrize("field", KZG_FIELDS)
@pytest.mark.parametrize("value", ["0x", "0x" + "00" * 47, "0x" + "00" * 49], ids=["empty", "47 bytes", "49 bytes"])
def test_blob_chunk_kzg_fields_must_be_48_bytes(rollup_request_and_validator, field, value) -> None:
    request, validator = rollup_request_and_validator
    request["proofRequest"]["chunks"][0][field] = value
    assert not validator.is_valid(request)


@pytest.mark.parametrize("field", KZG_FIELDS)
def test_chunk_kzg_fields_are_required(rollup_request_and_validator, field) -> None:
    request, validator = rollup_request_and_validator
    del request["proofRequest"]["chunks"][0][field]
    assert not validator.is_valid(request)


@pytest.fixture
def calldata_chunk_request_and_validator(rollup_request_and_validator):
    request, validator = rollup_request_and_validator
    request["proofRequest"]["chunks"][0].update(
        isCalldata=True, blobBytes="0x", calldataBytes="0x01", kzgCommitment="0x", kzgProof="0x",
    )
    return request, validator


def test_calldata_chunk_with_empty_kzg_fields_is_valid(calldata_chunk_request_and_validator) -> None:
    request, validator = calldata_chunk_request_and_validator
    validator.validate(request)


@pytest.mark.parametrize("field", KZG_FIELDS)
def test_calldata_chunk_kzg_fields_must_be_empty(calldata_chunk_request_and_validator, field) -> None:
    request, validator = calldata_chunk_request_and_validator
    request["proofRequest"]["chunks"][0][field] = "0x" + "00" * 48
    assert not validator.is_valid(request)


@dataclass(frozen=True)
class BlobTailCountCase:
    fixture_name: str
    count_path: tuple[str | int, ...]


_BLOB_TAIL_COUNT_CASES = [
    BlobTailCountCase("10-14-getZkRollupProofV1.request.json", ("proofRequest", "parentDataTailTakeBytes")),
    BlobTailCountCase("10-14-getZkRollupProofV1.response.json", ("publicInputs", "parentDataTailTakeBytes")),
    BlobTailCountCase("10-14-getZkRollupProofV1.response.json", ("publicInputs", "finalDataTailDiscardBytes")),
    BlobTailCountCase("10-18-getZkRollupAggregationProofV1.request.json", ("proofRequest", "rollupProofs", 0, "publicInputs", "parentDataTailTakeBytes")),
    BlobTailCountCase("10-18-getZkRollupAggregationProofV1.request.json", ("proofRequest", "rollupProofs", 0, "publicInputs", "finalDataTailDiscardBytes")),
    BlobTailCountCase("10-18-getZkRollupAggregationProofV1.response.json", ("publicInputs", "parentDataTailTakeBytes")),
    BlobTailCountCase("10-18-getZkRollupAggregationProofV1.response.json", ("publicInputs", "finalDataTailDiscardBytes")),
]


def _validate_blob_tail_count(case: BlobTailCountCase, value: int) -> None:
    jsonschema = pytest.importorskip("jsonschema")
    fixture_path = _FIXTURE_DIR / case.fixture_name
    schema = json.loads(_schema_path_for(fixture_path).read_text())
    validator = jsonschema.Draft202012Validator(schema)
    fixture = json.loads(fixture_path.read_text())
    target = fixture
    for key in case.count_path[:-1]:
        target = target[key]
    target[case.count_path[-1]] = value
    validator.validate(fixture)


@pytest.mark.parametrize("case", _BLOB_TAIL_COUNT_CASES)
def test_blob_tail_count_schema_accepts_maximum(case: BlobTailCountCase) -> None:
    _validate_blob_tail_count(case, 130046)


@pytest.mark.parametrize("case", _BLOB_TAIL_COUNT_CASES)
def test_blob_tail_count_schema_rejects_above_maximum(case: BlobTailCountCase) -> None:
    jsonschema = pytest.importorskip("jsonschema")
    with pytest.raises(jsonschema.ValidationError):
        _validate_blob_tail_count(case, 130047)
