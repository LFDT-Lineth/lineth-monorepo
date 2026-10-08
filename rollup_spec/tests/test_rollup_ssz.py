"""
Tests for the rollup SSZ wire format (`rollup_ssz.py`).

These cover the two properties this codec is responsible for:
  - round-trip fidelity: the input codec preserves the logical request; the
    output codec carries public inputs and their hash;
  - strict decoding: a wrong schema id, truncated bytes, or trailing bytes are
    all rejected rather than silently accepted or truncated.

There is no golden-vector byte-stability check here: pinning this encoder's
bytes against a fixture the same encoder produced would only be checking it
against itself. Implementations conform to this codec by round-tripping
against it directly.

Run from the rollup_spec/ directory:  python -m pytest
"""

import json
from pathlib import Path

import pytest

import rollup_spec
from ethereum.crypto.hash import Hash32, keccak256
from ethereum_types.numeric import U64

from rollup_spec.rollup import ChunkWitness, RollupProof
from rollup_spec.proof_io_v1 import _decode_rollup_public_input, decode_rollup_request
from rollup_spec.rollup_ssz import (
    decode_rollup_input_ssz,
    decode_rollup_output_ssz,
    encode_rollup_input,
    encode_rollup_output,
)
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


def _hexbytes(value: str) -> bytes:
    return bytes.fromhex(value[2:] if value[:2] in ("0x", "0X") else value)


def _rollup_output_from_response(resp: dict) -> RollupProof:
    """The rollup guest's own output implied by a response fixture: the same
    guest-emitted fields, with `proverVersion`/`proof` dropped."""
    pi = _decode_rollup_public_input(resp["publicInputs"], "publicInputs.")
    return RollupProof(
        public_inputs=pi,
        start_block_number=U64(resp["startBlockNumber"]),
    )


# ══════════════════════════════════════════════════════════════════════════════
# Round-trip: JSON fixture -> dataclass -> SSZ -> public inputs
# ══════════════════════════════════════════════════════════════════════════════


def test_rollup_input_round_trips_through_ssz() -> None:
    original = decode_rollup_request(_load_json("getZkRollupProofV1.request.json"))
    assert len(original.chunks[0].blob_bytes) == 131072
    original.chunks.append(
        ChunkWitness(
            Hash32(bytes([0x2A]) * 32),
            is_calldata=True,
            calldata_bytes=bytes(131_073),
        )
    )
    encoded = encode_rollup_input(original)
    recovered = decode_rollup_input_ssz(encoded)
    assert recovered == original


def test_rollup_output_frames_public_inputs_and_hash() -> None:
    response = _load_json("getZkRollupProofV1.response.json")
    original_output = _rollup_output_from_response(response)

    encoded = encode_rollup_output(original_output)
    pi_bytes = encoded[2:-32]
    assert encoded[:2] == (0x1801).to_bytes(2, "big")
    assert pi_bytes[:8] == int(original_output.public_inputs.end_block_number).to_bytes(8, "little")
    assert encoded[-32:] == keccak256(pi_bytes)
    assert decode_rollup_output_ssz(encoded) == original_output.public_inputs


def test_rollup_output_rejects_wrong_hash() -> None:
    encoded = bytearray(_rollup_output_bytes())
    encoded[-1] ^= 1
    with pytest.raises(InvalidSsz, match="hash mismatch"):
        decode_rollup_output_ssz(bytes(encoded))


def test_rollup_output_preserves_variable_length_public_inputs() -> None:
    proof = _rollup_output_from_response(_load_json("getZkRollupProofV1.response.json"))
    proof.public_inputs.l2_messaging_blocks_offsets = [3, 8]
    assert decode_rollup_output_ssz(encode_rollup_output(proof)) == proof.public_inputs


# ══════════════════════════════════════════════════════════════════════════════
# Strict decode rejections
# ══════════════════════════════════════════════════════════════════════════════
#
# Exercised once per decode function, corrupting bytes this module's own encoder just produced
# from the JSON fixtures — no checked-in SSZ fixture needed.

def _rollup_input_bytes() -> bytes:
    return encode_rollup_input(decode_rollup_request(_load_json("getZkRollupProofV1.request.json")))


def _rollup_output_bytes() -> bytes:
    return encode_rollup_output(_rollup_output_from_response(_load_json("getZkRollupProofV1.response.json")))


_DECODE_CASES = [
    pytest.param(decode_rollup_input_ssz, _rollup_input_bytes, 0x1001, id="rollup_input"),
    pytest.param(decode_rollup_output_ssz, _rollup_output_bytes, 0x1801, id="rollup_output"),
]


@pytest.mark.parametrize("decode_fn, encode_bytes, schema_id", _DECODE_CASES)
def test_decode_rejects_wrong_schema_id(decode_fn, encode_bytes, schema_id) -> None:
    encoded = bytearray(encode_bytes())
    # Flip the schema id to a value that is neither the expected id nor the
    # other schema id in this module.
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


@pytest.mark.parametrize("decode_fn, encode_bytes, schema_id", _DECODE_CASES)
def test_decode_rejects_trailing_garbage(decode_fn, encode_bytes, schema_id) -> None:
    # A trailing byte either breaks the outer container's own offset/length
    # bookkeeping (remerkleable raises directly) or decodes as if absorbed and
    # is then caught by the canonical-encoding re-check — either way it must
    # surface as InvalidSsz, not succeed silently.
    encoded = encode_bytes()
    with pytest.raises(InvalidSsz):
        decode_fn(encoded + b"\x00")


def test_decode_rollup_input_rejects_invalid_chunk_boolean() -> None:
    encoded = bytearray(_rollup_input_bytes())
    chunks_offset = int.from_bytes(encoded[2 + 52 : 2 + 56], "little")
    encoded[2 + chunks_offset + 4 + 32] = 2

    with pytest.raises(InvalidSsz):
        decode_rollup_input_ssz(bytes(encoded))


def test_decode_rollup_input_rejects_misaligned_chunk_container() -> None:
    encoded = bytearray(_rollup_input_bytes())
    chunks_offset = int.from_bytes(encoded[2 + 52 : 2 + 56], "little")
    proofs_offset_position = 2 + 56
    proofs_offset = int.from_bytes(encoded[proofs_offset_position : proofs_offset_position + 4], "little")
    del encoded[2 + chunks_offset + 40]
    encoded[proofs_offset_position : proofs_offset_position + 4] = (proofs_offset - 1).to_bytes(4, "little")
    for offset_position in (60, 64, 68):
        absolute_position = 2 + offset_position
        offset = int.from_bytes(encoded[absolute_position : absolute_position + 4], "little")
        encoded[absolute_position : absolute_position + 4] = (offset - 1).to_bytes(4, "little")

    with pytest.raises(InvalidSsz):
        decode_rollup_input_ssz(bytes(encoded))
