"""The committed `.ssz` fixtures are the spec's encodings of the JSON fixtures and decode back to their dataclasses."""

import json

import pytest

from rollup_spec.l2_execution_ssz import decode_l2_execution_input_ssz, decode_l2_execution_output_ssz
from rollup_spec.proof_io_v1 import (
    _decode_l2_execution_public_input,
    _decode_rollup_public_input,
    decode_request,
    decode_rollup_request,
)
from rollup_spec.rollup_ssz import decode_rollup_input_ssz, decode_rollup_output_ssz
from tests.ssz_fixtures import ENCODERS, L2_EXECUTION, ROLLUP, TESTDATA, encoded

DECODERS = {
    f"{L2_EXECUTION}.request": (decode_l2_execution_input_ssz, decode_request),
    f"{L2_EXECUTION}.response": (
        decode_l2_execution_output_ssz,
        lambda obj: _decode_l2_execution_public_input(obj["publicInputs"], "publicInputs."),
    ),
    f"{ROLLUP}.request": (decode_rollup_input_ssz, decode_rollup_request),
    f"{ROLLUP}.response": (
        decode_rollup_output_ssz,
        lambda obj: _decode_rollup_public_input(obj["publicInputs"], "publicInputs."),
    ),
}


@pytest.mark.parametrize("name", ENCODERS)
def test_committed_bytes_are_the_current_encoding(name):
    assert (TESTDATA / f"{name}.ssz").read_bytes() == encoded(name)


@pytest.mark.parametrize("name", ENCODERS)
def test_committed_bytes_decode_to_the_json_dataclass(name):
    decode_ssz, decode_json = DECODERS[name]

    from_ssz = decode_ssz((TESTDATA / f"{name}.ssz").read_bytes())

    assert from_ssz == decode_json(json.loads((TESTDATA / f"{name}.json").read_text()))
