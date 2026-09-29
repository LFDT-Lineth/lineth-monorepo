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


_BLOB_OFFSET_CASES = [
    ("10-14-getZkRollupProofV1.request.json", ("proofRequest", "startOffset")),
    ("10-14-getZkRollupProofV1.response.json", ("publicInputs", "startOffset")),
    ("10-14-getZkRollupProofV1.response.json", ("publicInputs", "endOffset")),
    ("10-18-getZkRollupAggregationProofV1.request.json", ("proofRequest", "rollupProofs", 0, "publicInputs", "startOffset")),
    ("10-18-getZkRollupAggregationProofV1.request.json", ("proofRequest", "rollupProofs", 0, "publicInputs", "endOffset")),
    ("10-18-getZkRollupAggregationProofV1.response.json", ("publicInputs", "startOffset")),
    ("10-18-getZkRollupAggregationProofV1.response.json", ("publicInputs", "endOffset")),
]


def _validate_blob_offset(fixture_name: str, offset_path: tuple[str | int, ...], value: int) -> None:
    jsonschema = pytest.importorskip("jsonschema")
    fixture_path = _FIXTURE_DIR / fixture_name
    schema = json.loads(_schema_path_for(fixture_path).read_text())
    validator = jsonschema.Draft202012Validator(schema)
    fixture = json.loads(fixture_path.read_text())
    target = fixture
    for key in offset_path[:-1]:
        target = target[key]
    target[offset_path[-1]] = value
    validator.validate(fixture)


@pytest.mark.parametrize(("fixture_name", "offset_path"), _BLOB_OFFSET_CASES)
def test_blob_offset_schema_accepts_last_payload_position(fixture_name: str, offset_path: tuple[str | int, ...]) -> None:
    _validate_blob_offset(fixture_name, offset_path, 130046)


@pytest.mark.parametrize(("fixture_name", "offset_path"), _BLOB_OFFSET_CASES)
def test_blob_offset_schema_rejects_position_past_payload(fixture_name: str, offset_path: tuple[str | int, ...]) -> None:
    jsonschema = pytest.importorskip("jsonschema")
    with pytest.raises(jsonschema.ValidationError):
        _validate_blob_offset(fixture_name, offset_path, 130047)
