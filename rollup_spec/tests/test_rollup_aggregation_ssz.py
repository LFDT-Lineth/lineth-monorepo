"""
Tests for the rollup-aggregation SSZ wire format (`rollup_aggregation_ssz.py`).

These cover the two properties this codec is responsible for:
  - round-trip fidelity: the SSZ codec preserves every field of the logical
    request/output dataclasses, and (for outputs) the JSON the coordinator
    would see back out;
  - strict decoding: a wrong schema id, truncated bytes, or trailing bytes are
    all rejected rather than silently accepted or truncated.

There is no golden-vector byte-stability check here: nothing outside this
package's own encoder consumes its exact byte output (the riscv-guests Zig
guests define and test their own wire format independently — see
`riscv-guests/rollup-aggregation/test/support.zig`), so pinning this
encoder's bytes against a checked-in fixture would only be checking it
against itself.

Run from the rollup_spec/ directory:  python -m pytest
"""

import json
from pathlib import Path

import pytest

import rollup_spec

from rollup_spec.l1_rollup import FinalizationPublicInput, FinalizationSubmission
from ethereum.crypto.hash import Hash32, keccak256
from rollup_spec.proof_io_v1 import (
    _decode_rollup_public_input,
    decode_aggregation_request,
    encode_aggregation_response,
)
from rollup_spec.rollup_aggregation_ssz import (
    ROLLUP_AGGREGATION_INPUT_SCHEMA_ID,
    ROLLUP_AGGREGATION_OUTPUT_SCHEMA_ID,
    SszFinalizationPublicInput,
    decode_aggregation_input_ssz,
    decode_aggregation_output_ssz,
    encode_aggregation_input,
    encode_aggregation_output,
)
from rollup_spec.stateless_input import InvalidSsz

_TESTDATA_DIR = Path(rollup_spec.__file__).resolve().parent / "prover_io" / "testdata"
_PROVER_VERSION = "4.0.0-riscv"


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


def _aggregation_output_from_response(resp: dict) -> FinalizationSubmission:
    """The rollup-aggregation guest's own output implied by a response fixture:
    the same guest-emitted fields, with `proverVersion`/`proof` dropped."""
    inputs = resp["publicInputs"]
    rollup_pi = _decode_rollup_public_input({**inputs, "programVks": [], "l2L1Messages": []}, "publicInputs.")
    pi = FinalizationPublicInput(
        **{name: getattr(rollup_pi, name) for name in FinalizationPublicInput.__dataclass_fields__
           if name not in ("program_ids", "l2_l1_roots", "l2_l1_tree_depth")},
        l2_l1_roots=[Hash32(_hexbytes(value)) for value in inputs["l2L1Roots"]],
        l2_l1_tree_depth=inputs["l2L1TreeDepth"],
        program_ids=[Hash32(_hexbytes(value)) for value in inputs["programIds"]],
    )
    return FinalizationSubmission(
        public_inputs=pi,
        proof=b"",
    )


# ══════════════════════════════════════════════════════════════════════════════
# Round-trip: JSON fixture -> dataclass -> SSZ -> dataclass (-> JSON)
# ══════════════════════════════════════════════════════════════════════════════


def test_aggregation_input_round_trips_through_ssz() -> None:
    original = decode_aggregation_request(
        _load_json("getZkRollupAggregationProofV1.request.json")
    )
    recovered = decode_aggregation_input_ssz(encode_aggregation_input(original))
    assert recovered == original


def test_aggregation_output_round_trips_through_ssz_and_back_to_json() -> None:
    response = _load_json("getZkRollupAggregationProofV1.response.json")
    original_output = _aggregation_output_from_response(response)

    recovered_output = decode_aggregation_output_ssz(encode_aggregation_output(original_output))
    assert recovered_output == original_output

    rebuilt_response = encode_aggregation_response(
        recovered_output,
        prover_version=_PROVER_VERSION,
        start_block_number=response["startBlockNumber"],
    )
    assert rebuilt_response == {**response, "proof": "0x"}


def test_aggregation_output_frames_public_inputs_directly() -> None:
    submission = _aggregation_output_from_response(_load_json("getZkRollupAggregationProofV1.response.json"))
    encoded = encode_aggregation_output(submission)
    body = encoded[34:]
    view = SszFinalizationPublicInput.decode_bytes(body)
    assert encoded[32:34] == ROLLUP_AGGREGATION_OUTPUT_SCHEMA_ID.to_bytes(2, "big")
    assert body[:8] == int(submission.public_inputs.end_block_number).to_bytes(8, "little")
    assert [bytes(program_id) for program_id in view.program_ids] == submission.public_inputs.program_ids
    assert [bytes(program_id) for program_id in view.program_ids] != [bytes([0xaa]) * 32, bytes([0xbb]) * 32]


def test_aggregation_output_commits_roots_and_rejects_tampering() -> None:
    submission = _aggregation_output_from_response(_load_json("getZkRollupAggregationProofV1.response.json"))
    original = encode_aggregation_output(submission)
    submission.public_inputs.l2_l1_roots = []
    modified = encode_aggregation_output(submission)
    assert modified[:32] != original[:32]
    assert decode_aggregation_output_ssz(modified).public_inputs.l2_l1_roots == []
    with pytest.raises(InvalidSsz, match="commitment"):
        decode_aggregation_output_ssz(original[:32] + modified[32:])


def test_aggregation_output_rejects_rehashed_wrong_schema_id() -> None:
    encoded = encode_aggregation_output(_aggregation_output_from_response(
        _load_json("getZkRollupAggregationProofV1.response.json")
    ))
    wrong_preimage = b"\xff\xfc" + encoded[34:]
    with pytest.raises(InvalidSsz, match="schema id"):
        decode_aggregation_output_ssz(keccak256(wrong_preimage) + wrong_preimage)


def test_aggregation_output_preserves_messaging_block_offsets() -> None:
    submission = _aggregation_output_from_response(
        _load_json("getZkRollupAggregationProofV1.response.json")
    )
    submission.public_inputs.l2_messaging_blocks_offsets = [3, 8]

    recovered = decode_aggregation_output_ssz(encode_aggregation_output(submission))
    response = encode_aggregation_response(
        recovered,
        prover_version=_PROVER_VERSION,
        start_block_number=10,
    )

    assert recovered.public_inputs.l2_messaging_blocks_offsets == [3, 8]
    assert response["publicInputs"]["l2MessagingBlocksOffsets"] == [3, 8]


# ══════════════════════════════════════════════════════════════════════════════
# Strict decode rejections
# ══════════════════════════════════════════════════════════════════════════════
#
# Exercised once per decode function, corrupting bytes this module's own encoder just produced
# from the JSON fixtures — no checked-in SSZ fixture needed.

def _aggregation_input_bytes() -> bytes:
    return encode_aggregation_input(
        decode_aggregation_request(_load_json("getZkRollupAggregationProofV1.request.json"))
    )


def _aggregation_output_bytes() -> bytes:
    return encode_aggregation_output(
        _aggregation_output_from_response(_load_json("getZkRollupAggregationProofV1.response.json"))
    )


_DECODE_CASES = [
    pytest.param(decode_aggregation_input_ssz, _aggregation_input_bytes, ROLLUP_AGGREGATION_INPUT_SCHEMA_ID, id="aggregation_input"),
    pytest.param(decode_aggregation_output_ssz, _aggregation_output_bytes, ROLLUP_AGGREGATION_OUTPUT_SCHEMA_ID, id="aggregation_output"),
]


@pytest.mark.parametrize("decode_fn, encode_bytes, schema_id", _DECODE_CASES)
def test_decode_rejects_wrong_schema_id(decode_fn, encode_bytes, schema_id) -> None:
    encoded = bytearray(encode_bytes())
    # Flip the schema id to a value that is neither the expected id nor any of
    # the other schema ids in the rollup/rollup-aggregation wire format.
    wrong_id = (schema_id ^ 0xFFFF).to_bytes(2, "big")
    encoded[32:34] = wrong_id if schema_id == ROLLUP_AGGREGATION_OUTPUT_SCHEMA_ID else encoded[32:34]
    if schema_id != ROLLUP_AGGREGATION_OUTPUT_SCHEMA_ID:
        encoded[0:2] = wrong_id
    with pytest.raises(InvalidSsz, match="schema id|commitment"):
        decode_fn(bytes(encoded))


@pytest.mark.parametrize("decode_fn, encode_bytes, schema_id", _DECODE_CASES)
def test_decode_rejects_malformed_truncation(decode_fn, encode_bytes, schema_id) -> None:
    encoded = encode_bytes()
    if schema_id == ROLLUP_AGGREGATION_INPUT_SCHEMA_ID:
        # The final proof is variable-length, so truncate the frame header.
        encoded = encoded[:3]
    else:
        encoded = encoded[:-1]
    with pytest.raises(InvalidSsz):
        decode_fn(encoded)


@pytest.mark.parametrize("decode_fn, encode_bytes, schema_id", _DECODE_CASES)
def test_decode_rejects_missing_schema_id(decode_fn, encode_bytes, schema_id) -> None:
    encoded = encode_bytes()
    with pytest.raises(InvalidSsz, match="schema id|commitment"):
        decode_fn(encoded[:1])


def test_decode_rejects_trailing_garbage_in_output() -> None:
    encoded = _aggregation_output_bytes()
    with pytest.raises(InvalidSsz):
        decode_aggregation_output_ssz(encoded + b"\x00")


def test_aggregation_input_accepts_variable_length_proof_bytes() -> None:
    original = decode_aggregation_input_ssz(_aggregation_input_bytes())
    original.rollup_proofs[-1].proof.proof += b"\x00"
    extended = decode_aggregation_input_ssz(encode_aggregation_input(original))
    original.rollup_proofs[-1].proof.proof = original.rollup_proofs[-1].proof.proof[:-2]
    shortened = decode_aggregation_input_ssz(encode_aggregation_input(original))
    assert shortened.rollup_proofs[-1].proof.proof == b"\xab\xcd"
    assert extended.rollup_proofs[-1].proof.proof == b"\xab\xcd\xff\x00"
