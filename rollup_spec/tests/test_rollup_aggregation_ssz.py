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
from ethereum.crypto.hash import Hash32
from ethereum.state import Address

from rollup_spec.l1_rollup import FinalizationSubmission
from rollup_spec.proof_io_v1 import (
    _decode_rollup_public_input,
    decode_aggregation_request,
    encode_aggregation_response,
)
from rollup_spec.rollup_aggregation_ssz import (
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
    pytest.param(decode_aggregation_input_ssz, _aggregation_input_bytes, 0x1002, id="aggregation_input"),
    pytest.param(decode_aggregation_output_ssz, _aggregation_output_bytes, 0x1802, id="aggregation_output"),
]


@pytest.mark.parametrize("decode_fn, encode_bytes, schema_id", _DECODE_CASES)
def test_decode_rejects_wrong_schema_id(decode_fn, encode_bytes, schema_id) -> None:
    encoded = bytearray(encode_bytes())
    # Flip the schema id to a value that is neither the expected id nor any of
    # the other schema ids in the rollup/rollup-aggregation wire format.
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
