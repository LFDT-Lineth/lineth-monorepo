"""
The SSZ contract fixtures: the bytes the spec's encoders produce for the JSON fixtures of `prover_io/testdata`.

Each `.ssz` file sits next to its JSON fixture. Requests are the framed guest inputs (`schema_id || ssz`); responses are
the framed guest outputs (`keccak256(frame) || frame`) of the response's public inputs.

From `rollup_spec/`, `PYTHONPATH=src python -m tests.ssz_fixtures` rewrites the `.ssz` files.
"""

import json
from pathlib import Path
from typing import Callable

from rollup_spec.l2_execution import L2ExecutionProof
from rollup_spec.l2_execution_ssz import encode_l2_execution_input, encode_l2_execution_output
from rollup_spec.proof_io_v1 import (
    _decode_l2_execution_public_input,
    _decode_rollup_public_input,
    decode_request,
    decode_rollup_request,
)
from rollup_spec.rollup import RollupProof
from rollup_spec.rollup_ssz import encode_rollup_input, encode_rollup_output

TESTDATA = Path(__file__).resolve().parents[1] / "src" / "rollup_spec" / "prover_io" / "testdata"

L2_EXECUTION = "10-11-getZkL2ExecutionProofV1"
ROLLUP = "10-14-getZkRollupProofV1"


def _l2_execution_output(response: dict) -> bytes:
    public_inputs = _decode_l2_execution_public_input(response["publicInputs"], "publicInputs.")
    return encode_l2_execution_output(L2ExecutionProof(public_inputs, response["startBlockNumber"], b""))


def _rollup_output(response: dict) -> bytes:
    public_inputs = _decode_rollup_public_input(response["publicInputs"], "publicInputs.")
    return encode_rollup_output(RollupProof(public_inputs, response["startBlockNumber"], b""))


ENCODERS: dict[str, Callable[[dict], bytes]] = {
    f"{L2_EXECUTION}.request": lambda obj: encode_l2_execution_input(decode_request(obj)),
    f"{L2_EXECUTION}.response": _l2_execution_output,
    f"{ROLLUP}.request": lambda obj: encode_rollup_input(decode_rollup_request(obj)),
    f"{ROLLUP}.response": _rollup_output,
}


def encoded(name: str) -> bytes:
    """The SSZ bytes for the JSON fixture `<name>.json`, `name` being e.g. `10-11-getZkL2ExecutionProofV1.request`."""
    return ENCODERS[name](json.loads((TESTDATA / f"{name}.json").read_text()))


if __name__ == "__main__":
    for fixture in ENCODERS:
        (TESTDATA / f"{fixture}.ssz").write_bytes(encoded(fixture))
