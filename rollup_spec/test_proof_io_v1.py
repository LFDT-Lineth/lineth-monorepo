"""
Round-trip and schema-conformance tests for the V1 JSON <-> guest-dataclass
codec (`proof_io_v1.py`).

The committed example files in `prover_inputs/` use `0x...` ellipsis placeholders
for documentation, so they are NOT valid fixtures. These tests load the
fully-valid fixtures under `prover_inputs/testdata/` (mutually consistent
request/response pair) and assert the codec round-trip against them.

Run from the repo root:  python -m pytest rollup_spec/test_proof_io_v1.py
"""

import json
from pathlib import Path

import pytest

from ethereum.crypto.hash import Hash32
from ethereum.state import Address
from ethereum_types.numeric import U64

from rollup_spec.block import ForcedTransactionAcceptance
from rollup_spec.l2_execution import (
    L2ExecutionProof,
    L2ExecutionProofPublicInput,
)
from rollup_spec.proof_io_v1 import (
    ProofIoError,
    decode_request,
    encode_response,
)

_SCHEMA_DIR = Path(__file__).parent / "schemas"
_TESTDATA_DIR = Path(__file__).parent / "prover_inputs" / "testdata"
_PROVER_VERSION = "4.0.0-riscv"


def _load(path: Path) -> dict:
    return json.loads(path.read_text())


def _valid_request() -> dict:
    return _load(_TESTDATA_DIR / "getZkL2ExecutionProofV1.request.json")


def _expected_response() -> dict:
    return _load(_TESTDATA_DIR / "getZkL2ExecutionProofV1.response.json")


def _sample_proof() -> L2ExecutionProof:
    pi = L2ExecutionProofPublicInput(
        parent_block_hash=Hash32(bytes([0x0A]) * 32),
        end_block_hash=Hash32(bytes([0x0B]) * 32),
        end_block_number=U64(1000503),
        end_block_timestamp=U64(1763000123),
        l2_l1_messages_hash=Hash32(bytes([0x01]) * 32),
        parent_l1_l2_bridge_rolling_hash=Hash32(bytes([0x02]) * 32),
        parent_l1_l2_bridge_rolling_hash_message_number=U64(0),
        end_l1_l2_bridge_rolling_hash=Hash32(bytes([0x03]) * 32),
        end_l1_l2_bridge_rolling_hash_message_number=U64(5),
        dynamic_chain_config_hash=Hash32(bytes([0xC0]) * 32),
        parent_ftx_rolling_hash=Hash32(bytes([0x04]) * 32),
        end_ftx_rolling_hash=Hash32(bytes([0x05]) * 32),
        last_processed_ftx_number=U64(18),
        filtered_addresses_hash=Hash32(bytes([0x06]) * 32),
        tx_froms_hash=Hash32(bytes([0x07]) * 32),
    )
    return L2ExecutionProof(
        public_inputs=pi,
        start_block_number=U64(1000501),
        end_block_number=U64(1000503),
        proof=b"\xde\xad\xbe\xef",
        l2_l1_messages=[Hash32(bytes([0x08]) * 32)],
        tx_froms=[Address(bytes([0x01]) * 20), Address(bytes([0x02]) * 20)],
        filtered_addresses=[Address(bytes([0x09]) * 20)],
    )


# ── request decode ─────────────────────────────────────────────────────────────


def test_decode_request_maps_all_fields_and_renames() -> None:
    req = decode_request(_valid_request())

    assert bytes(req.parent_ftx_rolling_hash) == bytes([0x0A]) * 32
    assert int(req.parent_last_processed_ftx_number) == 100

    assert bytes(req.chain_config.l2_message_service_address) == bytes([0x11]) * 20
    assert bytes(req.chain_config.coinbase) == bytes([0x00]) * 20
    assert int(req.chain_config.chain_id) == 59144

    assert len(req.payloads) == 2
    assert bytes(req.payloads[0].stateless_input_ssz) == bytes.fromhex("0001abcd")
    assert req.payloads[0].rollup_extension.forced_transactions == []
    assert bytes(req.payloads[1].stateless_input_ssz) == b""

    ftxs = req.payloads[1].rollup_extension.forced_transactions
    assert len(ftxs) == 2
    assert int(ftxs[0].number) == 17
    assert int(ftxs[0].deadline) == 1000600
    assert bytes(ftxs[0].signed_tx_rlp) == bytes.fromhex("02f86b")
    assert ftxs[0].acceptance == ForcedTransactionAcceptance.INCLUDED
    assert ftxs[1].acceptance == ForcedTransactionAcceptance.FILTERED_ADDRESS_TO
    assert bytes(ftxs[1].signed_tx_rlp) == b""


def test_debug_stateless_input_is_ignored() -> None:
    req = _valid_request()
    req["payloads"][0]["_debugStatelessInput"] = {"garbage": [1, 2, 3], "nested": {"x": "0xzz"}}
    # Decoding must succeed and not look at the mirror.
    decoded = decode_request(req)
    assert bytes(decoded.payloads[0].stateless_input_ssz) == bytes.fromhex("0001abcd")


def test_missing_required_field_is_rejected() -> None:
    req = _valid_request()
    del req["chainConfig"]["l2MessageServiceAddress"]
    with pytest.raises(ProofIoError, match="l2MessageServiceAddress"):
        decode_request(req)


def test_unknown_acceptance_is_rejected() -> None:
    req = _valid_request()
    req["payloads"][1]["rollupExtension"]["forcedTransactions"][0]["acceptance"] = "MAYBE"
    with pytest.raises(ProofIoError, match="acceptance"):
        decode_request(req)


def test_malformed_hex_is_rejected() -> None:
    req = _valid_request()
    req["parentFtxRollingHash"] = "0xnothex"
    with pytest.raises(ProofIoError, match="parentFtxRollingHash"):
        decode_request(req)


def test_non_hex_quantity_is_rejected() -> None:
    req = _valid_request()
    req["parentLastProcessedFtxNumber"] = "100"  # decimal string, not int / 0x-hex
    with pytest.raises(ProofIoError, match="parentLastProcessedFtxNumber"):
        decode_request(req)


def test_u64_accepts_hex_quantity() -> None:
    req = _valid_request()
    req["chainConfig"]["chainId"] = "0xe708"  # 59144
    decoded = decode_request(req)
    assert int(decoded.chain_config.chain_id) == 59144


# ── response encode ──────────────────────────────────────────────────────────


def test_encode_response_matches_fixture_exactly() -> None:
    # The testdata request/response pair is mutually consistent: _sample_proof()
    # is the L2ExecutionProof a guest run over the request fixture would yield,
    # so its encoding must equal the response fixture byte-for-byte (key set,
    # order-independent value comparison via dict equality).
    out = encode_response(_sample_proof(), prover_version=_PROVER_VERSION)
    assert out == _expected_response()


def test_encode_response_shape_and_values() -> None:
    out = encode_response(_sample_proof(), prover_version="4.0.0-riscv")

    assert out["proverVersion"] == "4.0.0-riscv"
    assert out["proof"] == "0xdeadbeef"
    assert out["startBlockNumber"] == 1000501
    assert out["endBlockNumber"] == 1000503

    pi = out["publicInputs"]
    assert pi["parentBlockHash"] == "0x" + ("0a" * 32)
    assert pi["endBlockHash"] == "0x" + ("0b" * 32)
    assert pi["endBlockNumber"] == 1000503
    assert pi["endBlockTimestamp"] == 1763000123
    assert pi["L2L1MessagesHash"] == "0x" + ("01" * 32)
    assert pi["endL1L2BridgeRollingHashMessageNumber"] == 5
    assert pi["lastProcessedFtxNumber"] == 18
    assert set(pi.keys()) == {
        "parentBlockHash", "endBlockHash", "endBlockNumber", "endBlockTimestamp",
        "L2L1MessagesHash", "parentL1L2BridgeRollingHash",
        "parentL1L2BridgeRollingHashMessageNumber", "endL1L2BridgeRollingHash",
        "endL1L2BridgeRollingHashMessageNumber", "dynamicChainConfigHash",
        "parentFtxRollingHash", "endFtxRollingHash", "lastProcessedFtxNumber",
        "filteredAddressesHash", "txFromsHash",
    }

    assert out["l2L1Messages"] == ["0x" + ("08" * 32)]
    assert out["txFroms"] == ["0x" + ("01" * 20), "0x" + ("02" * 20)]
    assert out["filteredAddresses"] == ["0x" + ("09" * 20)]


def test_empty_proof_bytes_encode_as_0x() -> None:
    proof = _sample_proof()
    proof.proof = b""
    out = encode_response(proof, prover_version="v")
    assert out["proof"] == "0x"


# ── JSON Schema conformance (shared cross-language contract) ───────────────────


def _validator(schema_name: str):
    jsonschema = pytest.importorskip("jsonschema")
    schema = json.loads((_SCHEMA_DIR / schema_name).read_text())
    Draft = jsonschema.Draft202012Validator
    Draft.check_schema(schema)
    return Draft(schema)


def test_valid_request_conforms_to_schema() -> None:
    _validator("getZkL2ExecutionProofV1.request.schema.json").validate(_valid_request())


def test_encoded_response_conforms_to_schema() -> None:
    out = encode_response(_sample_proof(), prover_version="4.0.0-riscv")
    _validator("getZkL2ExecutionProofV1.response.schema.json").validate(out)


def test_schema_rejects_bad_request() -> None:
    jsonschema = pytest.importorskip("jsonschema")
    validator = _validator("getZkL2ExecutionProofV1.request.schema.json")
    bad = _valid_request()
    bad["chainConfig"]["l2MessageServiceAddress"] = "0x1234"  # not 20 bytes
    with pytest.raises(jsonschema.ValidationError):
        validator.validate(bad)
