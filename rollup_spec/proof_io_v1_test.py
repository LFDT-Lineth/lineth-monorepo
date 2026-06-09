"""
Round-trip and schema-conformance tests for the V1 JSON <-> guest-dataclass
codec (`proof_io_v1.py`).

The committed example files in `prover_inputs/` use `0x...` ellipsis placeholders
for documentation, so they are NOT valid fixtures. These tests load the
fully-valid fixtures under `prover_inputs/testdata/` (mutually consistent
request/response pair) and assert the codec round-trip against them.

Run from the repo root:  python -m pytest rollup_spec/proof_io_v1_test.py
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
from rollup_spec.rollup import RollupProof, RollupPublicInput
from rollup_spec.proof_io_v1 import (
    ProofIoError,
    decode_aggregation_request,
    decode_aggregation_request_json,
    decode_request,
    decode_request_json,
    decode_rollup_request,
    decode_rollup_request_json,
    encode_aggregation_response,
    encode_response,
    encode_rollup_response,
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
    ftxs = req.payloads[0].rollup_extension.forced_transactions
    assert len(ftxs) == 1
    assert int(ftxs[0].number) == 16
    assert int(ftxs[0].deadline) == 1000599
    assert bytes(ftxs[0].signed_tx_rlp) == bytes.fromhex("02f86b")
    assert ftxs[0].acceptance == ForcedTransactionAcceptance.INCLUDED

    ftxs = req.payloads[1].rollup_extension.forced_transactions
    assert bytes(req.payloads[1].stateless_input_ssz) == bytes.fromhex("0001abcd")
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
    assert pi["l2L1MessagesHash"] == "0x" + ("01" * 32)
    assert pi["endL1L2BridgeRollingHashMessageNumber"] == 5
    assert pi["lastProcessedFtxNumber"] == 18
    assert set(pi.keys()) == {
        "parentBlockHash", "endBlockHash", "endBlockNumber", "endBlockTimestamp",
        "l2L1MessagesHash", "parentL1L2BridgeRollingHash",
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


# ══════════════════════════════════════════════════════════════════════════════
# Rollup proof (V1)
# ══════════════════════════════════════════════════════════════════════════════


def _valid_rollup_request() -> dict:
    return _load(_TESTDATA_DIR / "getZkRollupProofV1.request.json")


def _expected_rollup_response() -> dict:
    return _load(_TESTDATA_DIR / "getZkRollupProofV1.response.json")


def _sample_rollup_proof() -> RollupProof:
    pi = RollupPublicInput(
        end_block_number=U64(1000520),
        end_block_timestamp=U64(1763000457),
        l2_l1_bridge_transaction_tree=Hash32(bytes([0x11]) * 32),
        parent_l1_l2_bridge_rolling_hash=Hash32(bytes([0x22]) * 32),
        parent_l1_l2_bridge_rolling_hash_message_number=U64(0),
        end_l1_l2_bridge_rolling_hash=Hash32(bytes([0x33]) * 32),
        end_l1_l2_bridge_rolling_hash_message_number=U64(7),
        dynamic_chain_config_hash=Hash32(bytes([0xC0]) * 32),
        parent_ftx_rolling_hash=Hash32(bytes([0x44]) * 32),
        end_ftx_rolling_hash=Hash32(bytes([0x55]) * 32),
        last_processed_ftx_number=U64(9),
        filtered_addresses_hash=Hash32(bytes([0x66]) * 32),
        parent_shnarf=Hash32(bytes([0x47]) * 32),
        end_shnarf=Hash32(bytes([0x8D]) * 32),
    )
    return RollupProof(
        public_inputs=pi,
        start_block_number=U64(1000501),
        end_block_number=U64(1000520),
        proof=b"\xde\xad\xbe\xef",
        l2_l1_roots=[Hash32(bytes([0x77]) * 32), Hash32(bytes([0x88]) * 32)],
        filtered_addresses=[Address(bytes([0x01]) * 20)],
    )


# ── rollup request decode ──────────────────────────────────────────────────────


def test_decode_rollup_request_maps_all_fields() -> None:
    req = decode_rollup_request(_valid_rollup_request())

    assert int(req.chain_id) == 59144
    # shnarfTransition.parentShnarf -> parent_shnarf; endShnarf is metadata, dropped.
    assert bytes(req.parent_shnarf) == bytes([0x47]) * 32

    assert len(req.blobs) == 1
    blob = req.blobs[0]
    assert blob.block_number_range == (1000501, 1000510)
    assert bytes(blob.blob_hash) == bytes([0x1A]) * 32
    assert bytes(blob.blob_kzg_proof) == bytes([0x94]) * 48
    assert len(bytes(blob.blob_kzg_proof)) == 48
    assert blob.block_rlps == [bytes.fromhex("f90215a0"), bytes.fromhex("f90216b1")]

    assert len(req.l2_execution_proofs) == 1
    proof = req.l2_execution_proofs[0]
    assert bytes(proof.proof) == bytes.fromhex("abcdef")
    assert int(proof.start_block_number) == 1000501
    assert int(proof.end_block_number) == 1000510
    assert bytes(proof.public_inputs.parent_block_hash) == bytes([0x0A]) * 32
    assert bytes(proof.public_inputs.l2_l1_messages_hash) == bytes([0x01]) * 32
    assert int(proof.public_inputs.last_processed_ftx_number) == 12
    assert proof.l2_l1_messages == [Hash32(bytes([0x08]) * 32)]
    assert proof.tx_froms == [Address(bytes([0x01]) * 20), Address(bytes([0x02]) * 20)]
    assert proof.filtered_addresses == []


def test_decode_rollup_request_missing_field_is_rejected() -> None:
    req = _valid_rollup_request()
    del req["shnarfTransition"]["parentShnarf"]
    with pytest.raises(ProofIoError, match="parentShnarf"):
        decode_rollup_request(req)


def test_decode_rollup_request_empty_blobs_is_rejected() -> None:
    req = _valid_rollup_request()
    req["blobs"] = []
    with pytest.raises(ProofIoError, match="blobs"):
        decode_rollup_request(req)


def test_decode_rollup_request_empty_l2_execution_proofs_is_rejected() -> None:
    req = _valid_rollup_request()
    req["l2ExecutionProofs"] = []
    with pytest.raises(ProofIoError, match="l2ExecutionProofs"):
        decode_rollup_request(req)


def test_decode_rollup_request_non_array_blobs_is_rejected() -> None:
    req = _valid_rollup_request()
    req["blobs"] = {"not": "an array"}
    with pytest.raises(ProofIoError, match="blobs"):
        decode_rollup_request(req)


def test_decode_rollup_request_malformed_kzg_proof_is_rejected() -> None:
    req = _valid_rollup_request()
    req["blobs"][0]["blobInputs"]["blobKzgProof"] = "0xnothex"
    with pytest.raises(ProofIoError, match="blobKzgProof"):
        decode_rollup_request(req)


# ── rollup response encode ─────────────────────────────────────────────────────


def test_encode_rollup_response_matches_fixture_exactly() -> None:
    out = encode_rollup_response(_sample_rollup_proof(), prover_version=_PROVER_VERSION)
    assert out == _expected_rollup_response()


def test_encode_rollup_response_shape_and_values() -> None:
    out = encode_rollup_response(_sample_rollup_proof(), prover_version="4.0.0-riscv")

    assert out["proverVersion"] == "4.0.0-riscv"
    assert out["proof"] == "0xdeadbeef"
    assert out["startBlockNumber"] == 1000501
    assert out["endBlockNumber"] == 1000520

    pi = out["publicInputs"]
    assert pi["endBlockNumber"] == 1000520
    assert pi["endBlockTimestamp"] == 1763000457
    assert pi["l2L1BridgeTransactionTree"] == "0x" + ("11" * 32)
    assert pi["parentShnarf"] == "0x" + ("47" * 32)
    assert pi["endShnarf"] == "0x" + ("8d" * 32)
    assert pi["lastProcessedFtxNumber"] == 9
    assert set(pi.keys()) == {
        "endBlockNumber", "endBlockTimestamp", "l2L1BridgeTransactionTree",
        "parentL1L2BridgeRollingHash", "parentL1L2BridgeRollingHashMessageNumber",
        "endL1L2BridgeRollingHash", "endL1L2BridgeRollingHashMessageNumber",
        "dynamicChainConfigHash", "parentFtxRollingHash", "endFtxRollingHash",
        "lastProcessedFtxNumber", "filteredAddressesHash", "parentShnarf", "endShnarf",
    }

    assert out["l2L1Roots"] == ["0x" + ("77" * 32), "0x" + ("88" * 32)]
    assert out["filteredAddresses"] == ["0x" + ("01" * 20)]


# ── rollup JSON Schema conformance ─────────────────────────────────────────────


def test_valid_rollup_request_conforms_to_schema() -> None:
    _validator("getZkRollupProofV1.request.schema.json").validate(_valid_rollup_request())


def test_encoded_rollup_response_conforms_to_schema() -> None:
    out = encode_rollup_response(_sample_rollup_proof(), prover_version="4.0.0-riscv")
    _validator("getZkRollupProofV1.response.schema.json").validate(out)


def test_rollup_schema_rejects_bad_request() -> None:
    jsonschema = pytest.importorskip("jsonschema")
    validator = _validator("getZkRollupProofV1.request.schema.json")
    bad = _valid_rollup_request()
    bad["blobs"][0]["blobInputs"]["blobKzgProof"] = "0x1234"  # not 48 bytes
    with pytest.raises(jsonschema.ValidationError):
        validator.validate(bad)


# ══════════════════════════════════════════════════════════════════════════════
# Rollup-aggregation proof (V1)
# ══════════════════════════════════════════════════════════════════════════════


def _valid_aggregation_request() -> dict:
    return _load(_TESTDATA_DIR / "getZkRollupAggregationProofV1.request.json")


def _expected_aggregation_response() -> dict:
    return _load(_TESTDATA_DIR / "getZkRollupAggregationProofV1.response.json")


def _sample_rollup_public_input() -> RollupPublicInput:
    return RollupPublicInput(
        end_block_number=U64(1000520),
        end_block_timestamp=U64(1763000457),
        l2_l1_bridge_transaction_tree=Hash32(bytes([0x11]) * 32),
        parent_l1_l2_bridge_rolling_hash=Hash32(bytes([0x22]) * 32),
        parent_l1_l2_bridge_rolling_hash_message_number=U64(0),
        end_l1_l2_bridge_rolling_hash=Hash32(bytes([0x33]) * 32),
        end_l1_l2_bridge_rolling_hash_message_number=U64(7),
        dynamic_chain_config_hash=Hash32(bytes([0xC0]) * 32),
        parent_ftx_rolling_hash=Hash32(bytes([0x44]) * 32),
        end_ftx_rolling_hash=Hash32(bytes([0x55]) * 32),
        last_processed_ftx_number=U64(9),
        filtered_addresses_hash=Hash32(bytes([0x66]) * 32),
        parent_shnarf=Hash32(bytes([0x47]) * 32),
        end_shnarf=Hash32(bytes([0x8D]) * 32),
    )


# ── aggregation request decode ──────────────────────────────────────────────────


def test_decode_aggregation_request_maps_all_fields() -> None:
    req = decode_aggregation_request(_valid_aggregation_request())

    assert len(req.rollup_proofs) == 1
    proof = req.rollup_proofs[0]
    assert bytes(proof.proof) == bytes.fromhex("abcdef")
    assert int(proof.start_block_number) == 1000501
    assert int(proof.end_block_number) == 1000520
    assert proof.l2_l1_roots == [Hash32(bytes([0x77]) * 32), Hash32(bytes([0x88]) * 32)]
    assert proof.filtered_addresses == [Address(bytes([0x01]) * 20)]

    pi = proof.public_inputs
    assert int(pi.end_block_number) == 1000520
    assert int(pi.end_block_timestamp) == 1763000457
    assert bytes(pi.l2_l1_bridge_transaction_tree) == bytes([0x11]) * 32
    assert int(pi.end_l1_l2_bridge_rolling_hash_message_number) == 7
    assert int(pi.last_processed_ftx_number) == 9
    assert bytes(pi.parent_shnarf) == bytes([0x47]) * 32
    assert bytes(pi.end_shnarf) == bytes([0x8D]) * 32


def test_decode_aggregation_request_empty_rollup_proofs_is_rejected() -> None:
    req = _valid_aggregation_request()
    req["rollupProofs"] = []
    with pytest.raises(ProofIoError, match="rollupProofs"):
        decode_aggregation_request(req)


def test_decode_aggregation_request_non_array_rollup_proofs_is_rejected() -> None:
    req = _valid_aggregation_request()
    req["rollupProofs"] = {"not": "an array"}
    with pytest.raises(ProofIoError, match="rollupProofs"):
        decode_aggregation_request(req)


def test_decode_aggregation_request_missing_nested_pi_field_is_rejected() -> None:
    req = _valid_aggregation_request()
    del req["rollupProofs"][0]["publicInputs"]["endShnarf"]
    with pytest.raises(ProofIoError, match="endShnarf"):
        decode_aggregation_request(req)


def test_decode_aggregation_request_malformed_nested_hash_is_rejected() -> None:
    req = _valid_aggregation_request()
    req["rollupProofs"][0]["publicInputs"]["parentShnarf"] = "0xnothex"
    with pytest.raises(ProofIoError, match="parentShnarf"):
        decode_aggregation_request(req)


# ── aggregation response encode ─────────────────────────────────────────────────


def test_encode_aggregation_response_matches_fixture_exactly() -> None:
    out = encode_aggregation_response(
        _sample_rollup_public_input(),
        prover_version=_PROVER_VERSION,
        start_block_number=1000501,
        proof=b"\xde\xad\xbe\xef",
    )
    assert out == _expected_aggregation_response()


def test_encode_aggregation_response_shape_and_values() -> None:
    out = encode_aggregation_response(
        _sample_rollup_public_input(),
        prover_version="4.0.0-riscv",
        start_block_number=1000501,
        proof=b"\xde\xad\xbe\xef",
    )

    assert out["proverVersion"] == "4.0.0-riscv"
    assert out["proof"] == "0xdeadbeef"
    assert out["startBlockNumber"] == 1000501
    assert out["endBlockNumber"] == 1000520
    # The aggregation response has no top-level l2L1Roots / filteredAddresses.
    assert set(out.keys()) == {
        "proverVersion", "proof", "startBlockNumber", "endBlockNumber", "publicInputs",
    }

    pi = out["publicInputs"]
    assert pi["parentShnarf"] == "0x" + ("47" * 32)
    assert pi["endShnarf"] == "0x" + ("8d" * 32)
    assert pi["lastProcessedFtxNumber"] == 9
    assert set(pi.keys()) == {
        "endBlockNumber", "endBlockTimestamp", "l2L1BridgeTransactionTree",
        "parentL1L2BridgeRollingHash", "parentL1L2BridgeRollingHashMessageNumber",
        "endL1L2BridgeRollingHash", "endL1L2BridgeRollingHashMessageNumber",
        "dynamicChainConfigHash", "parentFtxRollingHash", "endFtxRollingHash",
        "lastProcessedFtxNumber", "filteredAddressesHash", "parentShnarf", "endShnarf",
    }


def test_encode_aggregation_response_defaults_proof_to_0x() -> None:
    out = encode_aggregation_response(
        _sample_rollup_public_input(), prover_version="v", start_block_number=1
    )
    assert out["proof"] == "0x"


# ── aggregation JSON Schema conformance ────────────────────────────────────────


def test_valid_aggregation_request_conforms_to_schema() -> None:
    _validator("getZkRollupAggregationProofV1.request.schema.json").validate(
        _valid_aggregation_request()
    )


def test_encoded_aggregation_response_conforms_to_schema() -> None:
    out = encode_aggregation_response(
        _sample_rollup_public_input(),
        prover_version="4.0.0-riscv",
        start_block_number=1000501,
        proof=b"\xde\xad\xbe\xef",
    )
    _validator("getZkRollupAggregationProofV1.response.schema.json").validate(out)


def test_aggregation_schema_rejects_bad_request() -> None:
    jsonschema = pytest.importorskip("jsonschema")
    validator = _validator("getZkRollupAggregationProofV1.request.schema.json")
    bad = _valid_aggregation_request()
    bad["rollupProofs"][0]["publicInputs"]["parentShnarf"] = "0x1234"  # not 32 bytes
    with pytest.raises(jsonschema.ValidationError):
        validator.validate(bad)


# ══════════════════════════════════════════════════════════════════════════════
# Opt-in schema enforcement on the `*_json` entry points (validate=True)
# ══════════════════════════════════════════════════════════════════════════════
#
# `decode_*_json(..., validate=True)` runs the JSON Schema before decoding. Its
# distinguishing power over the inline coercion is rejecting *unknown/extra*
# fields (the schemas set `additionalProperties: false`), which the decoders
# otherwise ignore. `jsonschema` is optional, so these tests skip without it.


def test_decode_request_json_validate_accepts_valid_request() -> None:
    pytest.importorskip("jsonschema")
    decoded = decode_request_json(json.dumps(_valid_request()), validate=True)
    assert int(decoded.chain_config.chain_id) == 59144


def test_decode_request_json_validate_rejects_unknown_field() -> None:
    pytest.importorskip("jsonschema")
    bad = _valid_request()
    bad["unexpectedField"] = 123
    with pytest.raises(ProofIoError, match="does not conform"):
        decode_request_json(json.dumps(bad), validate=True)


def test_decode_request_json_without_validate_ignores_unknown_field() -> None:
    # The inline coercion path only reads the keys it needs, so an extra field
    # is silently ignored when validate=False (the default).
    obj = _valid_request()
    obj["unexpectedField"] = 123
    decoded = decode_request_json(json.dumps(obj))  # validate=False
    assert int(decoded.chain_config.chain_id) == 59144


def test_decode_rollup_request_json_validate_rejects_unknown_field() -> None:
    pytest.importorskip("jsonschema")
    bad = _valid_rollup_request()
    bad["blobs"][0]["unexpectedField"] = "0xdead"
    with pytest.raises(ProofIoError, match="does not conform"):
        decode_rollup_request_json(json.dumps(bad), validate=True)


def test_decode_rollup_request_json_validate_accepts_valid_request() -> None:
    pytest.importorskip("jsonschema")
    decoded = decode_rollup_request_json(json.dumps(_valid_rollup_request()), validate=True)
    assert int(decoded.chain_id) == 59144


def test_decode_aggregation_request_json_validate_rejects_unknown_field() -> None:
    pytest.importorskip("jsonschema")
    bad = _valid_aggregation_request()
    bad["rollupProofs"][0]["publicInputs"]["unexpectedField"] = 1
    with pytest.raises(ProofIoError, match="does not conform"):
        decode_aggregation_request_json(json.dumps(bad), validate=True)


def test_decode_aggregation_request_json_validate_accepts_valid_request() -> None:
    pytest.importorskip("jsonschema")
    decoded = decode_aggregation_request_json(
        json.dumps(_valid_aggregation_request()), validate=True
    )
    assert len(decoded.rollup_proofs) == 1
