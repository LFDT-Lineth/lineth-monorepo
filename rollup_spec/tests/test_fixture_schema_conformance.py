"""
Standalone conformance test: every golden-vector fixture under
`rollup_spec/prover_io/testdata/` (both requests and responses) must validate
against its corresponding JSON Schema under `rollup_spec/prover_io/schemas/`.

This test does NOT import the guest dataclasses (only the lightweight,
dependency-free `rollup_spec` package root, to locate the data), so it has no
native dependencies (`ckzg`/`coincurve`/`lz4`) — only `jsonschema`. It runs on
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


def test_amsterdam_fixture_has_numeric_slots(execution_request_and_validator) -> None:
    request, validator = execution_request_and_validator
    assert request["proofRequest"]["chainConfig"]["forkName"] == "Amsterdam"
    assert [payload["slotNumber"] for payload in _execution_payloads(request)] == [10, 11]
    validator.validate(request)


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


@pytest.mark.parametrize("slot", [-1, 1.5, "10", "0xa", None, True])
def test_slot_requires_unsigned_integer(execution_request_and_validator, slot) -> None:
    request, validator = execution_request_and_validator
    _execution_payloads(request)[0]["slotNumber"] = slot
    errors = list(validator.iter_errors(request))
    assert any(list(error.path)[-1:] == ["slotNumber"] for error in errors)


def test_execution_payload_still_rejects_unknown_fields(execution_request_and_validator) -> None:
    request, validator = execution_request_and_validator
    _execution_payloads(request)[0]["unknownField"] = 1
    errors = list(validator.iter_errors(request))
    assert any(
        error.validator == "additionalProperties" and "unknownField" in error.message
        for error in errors
    )
