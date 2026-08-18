"""
Tests for the rollup / rollup-aggregation SSZ wire format (`rollup_ssz.py`).

These cover the three properties the guest decoders and a future Go encoder
rely on:
  - round-trip fidelity: the SSZ codec preserves every field of the logical
    request/output dataclasses, and (for outputs) the JSON the coordinator
    would see back out;
  - golden stability: encoding the fixtures reproduces the checked-in `.ssz.hex`
    bytes byte-for-byte, so an implementation in another language can assert
    its own encoder against the same vectors;
  - strict decoding: a wrong schema id, truncated bytes, or trailing bytes are
    all rejected rather than silently accepted or truncated.

Run from the rollup_spec/ directory:  python -m pytest
"""

import json
from pathlib import Path

import pytest

import rollup_spec
from ethereum.crypto.hash import Hash32
from ethereum.state import Address
from ethereum_types.numeric import U64

from rollup_spec.l1_rollup import FinalizationSubmission
from rollup_spec.rollup import RollupProof
from rollup_spec.proof_io_v1 import (
    _decode_rollup_public_input,
    decode_aggregation_request,
    encode_aggregation_response,
    decode_rollup_request,
    encode_rollup_response,
)
from rollup_spec.rollup_ssz import (
    ROLLUP_AGGREGATION_INPUT_SCHEMA_ID,
    ROLLUP_AGGREGATION_OUTPUT_SCHEMA_ID,
    ROLLUP_INPUT_SCHEMA_ID,
    ROLLUP_OUTPUT_SCHEMA_ID,
    InvalidSsz,
    decode_aggregation_input_ssz,
    decode_aggregation_output_ssz,
    decode_rollup_input_ssz,
    decode_rollup_output_ssz,
    encode_aggregation_input,
    encode_aggregation_output,
    encode_rollup_input,
    encode_rollup_output,
)

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


def _load_golden_ssz(name: str) -> bytes:
    """`name` is the `.ssz.hex` fixture's filename; the checked-in file is 0x-prefixed hex text,
    reviewable in a diff, rather than a raw binary blob."""
    return _hexbytes(_fixture(name).read_text().strip())


def _hexbytes(value: str) -> bytes:
    return bytes.fromhex(value[2:] if value[:2] in ("0x", "0X") else value)


def _rollup_output_from_response(resp: dict) -> RollupProof:
    """The rollup guest's own output implied by a response fixture: the same
    guest-emitted fields, with `proverVersion`/`proof` dropped."""
    pi = _decode_rollup_public_input(resp["publicInputs"], "publicInputs.")
    return RollupProof(
        public_inputs=pi,
        start_block_number=U64(resp["startBlockNumber"]),
        l2_l1_roots=[Hash32(_hexbytes(h)) for h in resp["l2L1Roots"]],
        filtered_addresses=[Address(_hexbytes(a)) for a in resp["filteredAddresses"]],
    )


def _aggregation_output_from_response(resp: dict) -> FinalizationSubmission:
    """The rollup-aggregation guest's own output implied by a response fixture:
    the same guest-emitted fields, with `proverVersion`/`proof` dropped."""
    pi = _decode_rollup_public_input(resp["publicInputs"], "publicInputs.")
    return FinalizationSubmission(
        public_inputs=pi,
        proof=b"",
        l2_l1_roots=[Hash32(_hexbytes(h)) for h in resp["l2L1Roots"]],
        filtered_addresses=[Address(_hexbytes(a)) for a in resp["filteredAddresses"]],
        l2_messaging_blocks_offsets=list(resp["l2MessagingBlocksOffsets"]),
    )


# ══════════════════════════════════════════════════════════════════════════════
# Round-trip: JSON fixture -> dataclass -> SSZ -> dataclass (-> JSON)
# ══════════════════════════════════════════════════════════════════════════════


def test_rollup_input_round_trips_through_ssz() -> None:
    original = decode_rollup_request(_load_json("getZkRollupProofV1.request.json"))
    recovered = decode_rollup_input_ssz(encode_rollup_input(original))
    assert recovered == original


def test_aggregation_input_round_trips_through_ssz() -> None:
    original = decode_aggregation_request(
        _load_json("getZkRollupAggregationProofV1.request.json")
    )
    recovered = decode_aggregation_input_ssz(encode_aggregation_input(original))
    assert recovered == original


def test_rollup_output_round_trips_through_ssz_and_back_to_json() -> None:
    response = _load_json("getZkRollupProofV1.response.json")
    original_output = _rollup_output_from_response(response)

    recovered_output = decode_rollup_output_ssz(encode_rollup_output(original_output))
    assert recovered_output == original_output

    # The guest never emits `proof`; round-tripping through the JSON encoder
    # reproduces the original response with `proof` reset to the placeholder.
    rebuilt_response = encode_rollup_response(recovered_output, prover_version=_PROVER_VERSION)
    assert rebuilt_response == {**response, "proof": "0x"}


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


# ══════════════════════════════════════════════════════════════════════════════
# Golden stability: encoding the fixtures reproduces the checked-in bytes
# ══════════════════════════════════════════════════════════════════════════════


def test_rollup_input_matches_golden_vector() -> None:
    original = decode_rollup_request(_load_json("getZkRollupProofV1.request.json"))
    assert encode_rollup_input(original) == _load_golden_ssz("getZkRollupProofV1.request.ssz.hex")


def test_aggregation_input_matches_golden_vector() -> None:
    original = decode_aggregation_request(
        _load_json("getZkRollupAggregationProofV1.request.json")
    )
    assert encode_aggregation_input(original) == _load_golden_ssz(
        "getZkRollupAggregationProofV1.request.ssz.hex"
    )


def test_rollup_output_matches_golden_vector() -> None:
    original_output = _rollup_output_from_response(
        _load_json("getZkRollupProofV1.response.json")
    )
    assert encode_rollup_output(original_output) == _load_golden_ssz(
        "getZkRollupProofV1.response.ssz.hex"
    )


def test_aggregation_output_matches_golden_vector() -> None:
    original_output = _aggregation_output_from_response(
        _load_json("getZkRollupAggregationProofV1.response.json")
    )
    assert encode_aggregation_output(original_output) == _load_golden_ssz(
        "getZkRollupAggregationProofV1.response.ssz.hex"
    )


# ══════════════════════════════════════════════════════════════════════════════
# Strict decode rejections
# ══════════════════════════════════════════════════════════════════════════════
#
# Exercised once per decode function, against that function's own golden
# vector, so each case is a real framed message rather than synthetic bytes.

_DECODE_CASES = [
    pytest.param(
        decode_rollup_input_ssz,
        "getZkRollupProofV1.request.ssz.hex",
        ROLLUP_INPUT_SCHEMA_ID,
        id="rollup_input",
    ),
    pytest.param(
        decode_aggregation_input_ssz,
        "getZkRollupAggregationProofV1.request.ssz.hex",
        ROLLUP_AGGREGATION_INPUT_SCHEMA_ID,
        id="aggregation_input",
    ),
    pytest.param(
        decode_rollup_output_ssz,
        "getZkRollupProofV1.response.ssz.hex",
        ROLLUP_OUTPUT_SCHEMA_ID,
        id="rollup_output",
    ),
    pytest.param(
        decode_aggregation_output_ssz,
        "getZkRollupAggregationProofV1.response.ssz.hex",
        ROLLUP_AGGREGATION_OUTPUT_SCHEMA_ID,
        id="aggregation_output",
    ),
]


@pytest.mark.parametrize("decode_fn, golden_name, schema_id", _DECODE_CASES)
def test_decode_rejects_wrong_schema_id(decode_fn, golden_name, schema_id) -> None:
    golden = bytearray(_load_golden_ssz(golden_name))
    # Flip the schema id to a value that is neither the expected id nor any of
    # the other three schema ids in this module.
    wrong_id = (schema_id ^ 0xFFFF).to_bytes(2, "big")
    golden[0:2] = wrong_id
    with pytest.raises(InvalidSsz, match="schema id"):
        decode_fn(bytes(golden))


@pytest.mark.parametrize("decode_fn, golden_name, schema_id", _DECODE_CASES)
def test_decode_rejects_truncated_bytes(decode_fn, golden_name, schema_id) -> None:
    golden = _load_golden_ssz(golden_name)
    with pytest.raises(InvalidSsz):
        decode_fn(golden[: len(golden) - 1])


@pytest.mark.parametrize("decode_fn, golden_name, schema_id", _DECODE_CASES)
def test_decode_rejects_missing_schema_id(decode_fn, golden_name, schema_id) -> None:
    golden = _load_golden_ssz(golden_name)
    with pytest.raises(InvalidSsz, match="schema id"):
        decode_fn(golden[:1])


@pytest.mark.parametrize("decode_fn, golden_name, schema_id", _DECODE_CASES)
def test_decode_rejects_trailing_garbage(decode_fn, golden_name, schema_id) -> None:
    # A trailing byte either breaks the outer container's own offset/length
    # bookkeeping (remerkleable raises directly) or decodes as if absorbed and
    # is then caught by the canonical-encoding re-check — either way it must
    # surface as InvalidSsz, not succeed silently.
    golden = _load_golden_ssz(golden_name)
    with pytest.raises(InvalidSsz):
        decode_fn(golden + b"\x00")
