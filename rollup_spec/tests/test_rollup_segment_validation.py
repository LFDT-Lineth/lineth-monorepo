"""Unit tests for the rollup guest's witnessed zstd segment validation."""

from dataclasses import replace
import json
from pathlib import Path

import pytest
import zstandard as zstd
from ethereum.crypto.hash import Hash32, keccak256
from ethereum_types.numeric import U64

from rollup_spec.rollup import (
    ChunkWitness,
    ConflationWitness,
    RollupProofPrivateInput,
    _validate_conflation_segment,
    _verify_and_fold_chunks,
    run_rollup_guest,
)
from rollup_spec import rollup
import rollup_spec
from rollup_spec.l2_execution import L2ExecutionProof, L2ExecutionProofPublicInput, VerifiableL2ExecutionProof
from rollup_spec.proof_io_v1 import decode_rollup_request


def _segment(payload: bytes) -> bytes:
    return zstd.ZstdCompressor().compress(payload)


def test_validate_conflation_segment_accepts_matching_zstd_frame() -> None:
    expected = b"canonical truncated-block RLP"

    _validate_conflation_segment(_segment(expected), expected)


def test_validate_conflation_segment_rejects_invalid_zstd_data() -> None:
    with pytest.raises(Exception, match="invalid zstd"):
        _validate_conflation_segment(b"bad", b"payload")


def test_validate_conflation_segment_rejects_trailing_zstd_data() -> None:
    with pytest.raises(Exception, match="invalid zstd"):
        _validate_conflation_segment(_segment(b"payload") + b"trailing", b"payload")


def test_validate_conflation_segment_rejects_second_frame() -> None:
    with pytest.raises(Exception, match="invalid zstd"):
        _validate_conflation_segment(_segment(b"payload") + _segment(b"other"), b"payload")


def test_validate_conflation_segment_rejects_truncated_frame() -> None:
    with pytest.raises(Exception, match="invalid zstd"):
        _validate_conflation_segment(_segment(b"payload")[:-1], b"payload")


def test_validate_conflation_segment_rejects_mismatched_decompressed_bytes() -> None:
    with pytest.raises(Exception, match="does not match"):
        _validate_conflation_segment(_segment(b"other!!"), b"payload")


def test_fixture_frames_have_distinct_encodings_and_derive_stream_prefixes() -> None:
    fixture = Path(rollup_spec.__file__).parent / "prover_io" / "testdata" / "10-14-getZkRollupProofV1.request.json"
    request = decode_rollup_request(json.loads(fixture.read_text()))
    frames = [conflation.compressed_segment for conflation in request.conflations]
    assert frames[0] != frames[1]
    assert [zstd.ZstdDecompressor().decompress(frame, max_output_size=17, allow_extra_data=False) for frame in frames] == [
        b"canonical payload", b"canonical payload",
    ]
    stream = b"".join(len(frame).to_bytes(4, "big") + frame for frame in frames)
    assert stream == (
        bytes.fromhex("0000001a") + frames[0] + bytes.fromhex("0000001a") + frames[1]
    )
    parent_hash = Hash32(bytes(32))
    _, end_offset = _verify_and_fold_chunks(
        stream, 0,
        [ChunkWitness(Hash32(keccak256(stream)), is_calldata=True, calldata_length=len(stream))],
        b"", b"", parent_hash, None, [4 + len(frames[0]), len(stream)],
    )
    assert end_offset == 0
    with pytest.raises(Exception, match="computed hash does not match chunkHash"):
        _verify_and_fold_chunks(
            stream, 0,
            [ChunkWitness(Hash32(keccak256(b"".join(frames))), is_calldata=True, calldata_length=len(stream))],
            b"", b"", parent_hash, None, [4 + len(frames[0]), len(stream)],
        )


def test_rollup_stream_uses_exact_witnessed_frames_with_big_endian_lengths(monkeypatch) -> None:
    payload = b"canonical truncated-block RLP"
    frames = [
        zstd.ZstdCompressor().compress(payload),
        zstd.ZstdCompressor(write_content_size=False).compress(payload),
    ]
    assert frames[0] != frames[1]
    expected_segments = [len(frame).to_bytes(4, "big") + frame for frame in frames]
    expected_stream = b"".join(expected_segments)
    offsets = [len(expected_segments[0]), len(expected_stream)]
    zero = Hash32(bytes(32))
    pi = L2ExecutionProofPublicInput(
        parent_block_hash=zero, end_block_hash=zero, end_block_number=U64(1),
        end_block_timestamp=U64(1), l2_l1_messages_hash=zero,
        parent_l1_l2_bridge_rolling_hash=zero, parent_l1_l2_bridge_rolling_hash_message_number=U64(0),
        end_l1_l2_bridge_rolling_hash=zero, end_l1_l2_bridge_rolling_hash_message_number=U64(0),
        dynamic_chain_config_hash=zero, parent_ftx_rolling_hash=zero, parent_ftx_number=U64(0),
        end_ftx_rolling_hash=zero, end_processed_ftx_number=U64(0),
        filtered_addresses_hash=zero, tx_froms_hash=zero,
    )
    proofs = [VerifiableL2ExecutionProof(
        proof=L2ExecutionProof(public_inputs=replace(pi, end_block_number=U64(i)), start_block_number=U64(i)),
        program_vk=zero,
    ) for i in (1, 2)]
    guest_input = RollupProofPrivateInput(
        parent_data_rolling_hash=zero, start_offset=0, chain_id=U64(1),
        conflations=[ConflationWitness([b"block"], frame) for frame in frames],
        chunks=[ChunkWitness(zero, is_calldata=True, calldata_length=len(expected_stream))],
        l2_execution_proofs=proofs,
    )
    monkeypatch.setattr(rollup, "_truncate_conflation", lambda blocks, chain_id: ([object()], [zero]))
    monkeypatch.setattr(rollup, "rlp_encode_truncated_blocks", lambda blocks: payload)

    def check_stream(own_stream_bytes, start_offset, chunks, prefix, suffix, parent_hash, boundary_hash, segment_end_offsets):
        assert own_stream_bytes == expected_stream
        assert segment_end_offsets == offsets
        raise RuntimeError("stream inspected")

    monkeypatch.setattr(rollup, "_verify_and_fold_chunks", check_stream)
    with pytest.raises(RuntimeError, match="stream inspected"):
        run_rollup_guest(guest_input)
