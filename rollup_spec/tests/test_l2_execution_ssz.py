"""
Tests for the l2-execution SSZ wire format (`l2_execution_ssz.py`).

These cover the properties this codec is responsible for:
  - round-trip fidelity: the SSZ codec preserves every field of the logical
    guest-input dataclass, including the nested payloads, forced-transaction
    witnesses, and the opaque per-payload stateless-input byte slices;
  - output framing: the 0x0003 frame carries exactly the keccak256 of the
    fixed 368-byte SSZ public-input tuple and nothing else;
  - strict decoding: a wrong schema id, truncated bytes, or trailing bytes are
    all rejected rather than silently accepted or truncated.

Run from the rollup_spec/ directory:  python -m pytest
"""

import json
from pathlib import Path

import pytest

import rollup_spec
from ethereum.crypto.hash import keccak256
from ethereum_types.numeric import U64

from rollup_spec.l2_execution import L2ExecutionProof
from rollup_spec.l2_execution_ssz import (
    decode_l2_execution_input_ssz,
    decode_l2_execution_output_ssz,
    encode_l2_execution_input,
    encode_l2_execution_output,
    encode_l2_execution_public_inputs_bytes,
)
from rollup_spec.proof_io_v1 import _decode_l2_execution_public_input, decode_request
from rollup_spec.stateless_input import InvalidSsz

_TESTDATA_DIR = Path(rollup_spec.__file__).resolve().parent / "prover_io" / "testdata"


def _fixture(name: str) -> Path:
    """Resolve `<name>`, allowing an optional `<startBlock>-<endBlock>-` prefix."""
    matches = sorted(_TESTDATA_DIR.glob(f"*{name}"))
    assert matches, f"no fixture matching *{name} in {_TESTDATA_DIR}"
    assert len(matches) == 1, f"multiple fixtures matching *{name}: {matches}"
    return matches[0]


def _load_json(name: str) -> dict:
    return json.loads(_fixture(name).read_text())


def _l2_execution_input():
    return decode_request(_load_json("getZkL2ExecutionProofV1.request.json"))


def _l2_execution_proof() -> L2ExecutionProof:
    """The guest output implied by the response fixture (the preimage lists
    are irrelevant here: only `public_inputs` reaches the 0x0003 wire)."""
    response = _load_json("getZkL2ExecutionProofV1.response.json")
    return L2ExecutionProof(
        public_inputs=_decode_l2_execution_public_input(response["publicInputs"], "publicInputs."),
        start_block_number=U64(response["startBlockNumber"]),
    )


# ══════════════════════════════════════════════════════════════════════════════
# Round-trip: JSON fixture -> dataclass -> SSZ -> dataclass
# ══════════════════════════════════════════════════════════════════════════════


def test_l2_execution_input_round_trips_through_ssz() -> None:
    original = _l2_execution_input()
    recovered = decode_l2_execution_input_ssz(encode_l2_execution_input(original))
    assert recovered == original


def test_l2_execution_input_fixture_exercises_nested_fields() -> None:
    # Guard the round-trip test's coverage: the fixture must exercise the
    # nested payload list and the opaque stateless-input slices, or the
    # round-trip would vacuously pass on empty containers.
    original = _l2_execution_input()
    assert len(original.payloads) > 0
    assert all(len(p.stateless_input_ssz) > 0 for p in original.payloads)


# ══════════════════════════════════════════════════════════════════════════════
# Output: hash-only 0x0003 frame
# ══════════════════════════════════════════════════════════════════════════════


def test_l2_execution_output_is_a_34_byte_hash_frame() -> None:
    encoded = encode_l2_execution_output(_l2_execution_proof())
    assert len(encoded) == 34
    assert encoded[:2] == (0x0003).to_bytes(2, "big")


def test_l2_execution_output_hash_is_keccak_of_the_public_input_tuple() -> None:
    proof = _l2_execution_proof()
    encoded = encode_l2_execution_output(proof)
    expected = keccak256(encode_l2_execution_public_inputs_bytes(proof.public_inputs))
    assert encoded[2:] == expected
    assert decode_l2_execution_output_ssz(encoded) == expected


def test_l2_execution_public_input_tuple_encodes_to_exactly_368_bytes() -> None:
    # Pins the fixed tuple layout (16 fields: 10 x Bytes32 + 6 x uint64) the
    # guest hashes; any field addition/reorder/retype changes this size.
    proof = _l2_execution_proof()
    assert len(encode_l2_execution_public_inputs_bytes(proof.public_inputs)) == 368


# ══════════════════════════════════════════════════════════════════════════════
# Strict decode rejections
# ══════════════════════════════════════════════════════════════════════════════
#
# Exercised on bytes this module's own encoder just produced from the JSON
# fixtures — no checked-in SSZ fixture needed.


def _l2_execution_input_bytes() -> bytes:
    return encode_l2_execution_input(_l2_execution_input())


def _l2_execution_output_bytes() -> bytes:
    return encode_l2_execution_output(_l2_execution_proof())


_DECODE_CASES = [
    pytest.param(
        decode_l2_execution_input_ssz, _l2_execution_input_bytes, 0x0002, id="l2_execution_input"
    ),
    pytest.param(
        decode_l2_execution_output_ssz, _l2_execution_output_bytes, 0x0003, id="l2_execution_output"
    ),
]


@pytest.mark.parametrize("decode_fn, encode_bytes, schema_id", _DECODE_CASES)
def test_decode_rejects_wrong_schema_id(decode_fn, encode_bytes, schema_id) -> None:
    encoded = bytearray(encode_bytes())
    # Flip the schema id to a value that is not any schema id defined across
    # the guest wire modules.
    wrong_id = (schema_id ^ 0xFFFF).to_bytes(2, "big")
    encoded[0:2] = wrong_id
    with pytest.raises(InvalidSsz, match="schema id"):
        decode_fn(bytes(encoded))


@pytest.mark.parametrize("decode_fn, encode_bytes, schema_id", _DECODE_CASES)
def test_decode_rejects_truncated_bytes(decode_fn, encode_bytes, schema_id) -> None:
    encoded = encode_bytes()
    with pytest.raises(InvalidSsz):
        decode_fn(encoded[: len(encoded) - 1])


@pytest.mark.parametrize("decode_fn, encode_bytes, schema_id", _DECODE_CASES)
def test_decode_rejects_missing_schema_id(decode_fn, encode_bytes, schema_id) -> None:
    encoded = encode_bytes()
    with pytest.raises(InvalidSsz, match="schema id"):
        decode_fn(encoded[:1])


def test_input_decode_never_silently_absorbs_a_trailing_byte() -> None:
    # SSZ encoding is bijective, so a trailing byte yields either invalid SSZ
    # or the canonical encoding of a DIFFERENT value. The input container ends
    # in a nested byte list, so the appended byte lands inside the last
    # payload's final `signed_tx_rlp` — a changed value, hence the second
    # branch: the decode must never hand back the original value as if the
    # byte were ignorable framing slack.
    encoded = _l2_execution_input_bytes()
    original = decode_l2_execution_input_ssz(encoded)
    try:
        mutated = decode_l2_execution_input_ssz(encoded + b"\x00")
    except InvalidSsz:
        return
    assert mutated != original


def test_output_decode_rejects_trailing_garbage() -> None:
    # The output body is fixed-size (one 32-byte hash), so trailing bytes are
    # always detectable and must be rejected outright.
    encoded = _l2_execution_output_bytes()
    with pytest.raises(InvalidSsz):
        decode_l2_execution_output_ssz(encoded + b"\x00")
