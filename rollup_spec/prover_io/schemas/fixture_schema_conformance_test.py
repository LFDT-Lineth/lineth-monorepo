"""
Standalone conformance test: every fixture under `fixture/` must validate against
its corresponding JSON Schema in this directory.

This test does NOT import the guest dataclasses, so it has no native
dependencies (`ckzg`/`coincurve`/`lz4`) — only `jsonschema`. It runs on any
Python and is the cheapest way to catch a fixture drifting from its schema.

Fixture <-> schema pairing is by filename convention:

    fixture/<name>.json   <->   <name>.schema.json

Fixtures are discovered automatically, so a new fixture/schema pair is covered
without editing this file.

Run from the repo root:
    python -m pytest rollup_spec/prover_io/schemas/fixture_schema_conformance_test.py
"""

import json
from pathlib import Path

import pytest

_SCHEMA_DIR = Path(__file__).parent
_FIXTURE_DIR = _SCHEMA_DIR / "fixture"


def _schema_path_for(fixture_path: Path) -> Path:
    """fixture/<name>.json -> <name>.schema.json (sibling of this test)."""
    return _SCHEMA_DIR / f"{fixture_path.name[: -len('.json')]}.schema.json"


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
