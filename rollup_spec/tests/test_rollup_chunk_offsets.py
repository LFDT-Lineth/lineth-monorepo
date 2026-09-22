import pytest
from ethereum.crypto.hash import Hash32, keccak256

from rollup_spec import rollup
from rollup_spec.rollup import BLOB_BYTES_LENGTH, ChunkWitness, _verify_and_fold_chunks


_PARENT_HASH = Hash32(bytes([0x11]) * 32)
_CHUNK_HASH = Hash32(bytes([0x22]) * 32)


def _fold_blob(monkeypatch, own_bytes: bytes, suffix: bytes) -> int:
    monkeypatch.setattr(rollup, "_trusted_setup", lambda: object())
    monkeypatch.setattr(rollup.ckzg, "blob_to_kzg_commitment", lambda blob, setup: bytes(48))
    monkeypatch.setattr(rollup, "kzg_commitment_to_versioned_hash", lambda commitment: _CHUNK_HASH)
    _, end_offset = _verify_and_fold_chunks(
        own_stream_bytes=own_bytes,
        start_offset=0,
        chunks=[ChunkWitness(_CHUNK_HASH, is_calldata=False, calldata_length=0)],
        opaque_prefix_bytes=b"",
        opaque_suffix_bytes=suffix,
        parent_data_rolling_hash=_PARENT_HASH,
        boundary_prev_data_rolling_hash=None,
        segment_end_offsets=[len(own_bytes)],
    )
    return end_offset


def test_terminal_calldata_chunk_uses_canonical_boundary_offset() -> None:
    own_bytes = b"calldata-segment"
    _, end_offset = _verify_and_fold_chunks(
        own_stream_bytes=own_bytes,
        start_offset=0,
        chunks=[ChunkWitness(Hash32(keccak256(own_bytes)), is_calldata=True, calldata_length=len(own_bytes))],
        opaque_prefix_bytes=b"",
        opaque_suffix_bytes=b"",
        parent_data_rolling_hash=_PARENT_HASH,
        boundary_prev_data_rolling_hash=None,
        segment_end_offsets=[len(own_bytes)],
    )

    assert end_offset == 0


def _fold_calldata(own_bytes: bytes, chunk: ChunkWitness, segment_end_offsets: list[int]) -> int:
    _, end_offset = _verify_and_fold_chunks(
        own_stream_bytes=own_bytes,
        start_offset=0,
        chunks=[chunk],
        opaque_prefix_bytes=b"",
        opaque_suffix_bytes=b"",
        parent_data_rolling_hash=_PARENT_HASH,
        boundary_prev_data_rolling_hash=None,
        segment_end_offsets=segment_end_offsets,
    )
    return end_offset


def test_calldata_chunk_can_exceed_blob_size() -> None:
    own_bytes = bytes(BLOB_BYTES_LENGTH + 1)
    chunk = ChunkWitness(Hash32(keccak256(own_bytes)), is_calldata=True, calldata_length=len(own_bytes))

    assert _fold_calldata(own_bytes, chunk, [len(own_bytes)]) == 0


def test_blob_chunk_rejects_nonzero_calldata_length(monkeypatch) -> None:
    monkeypatch.setattr(rollup, "_trusted_setup", lambda: object())
    with pytest.raises(Exception, match="blob chunk 0 must have calldataLength 0"):
        _verify_and_fold_chunks(
            own_stream_bytes=bytes(BLOB_BYTES_LENGTH),
            start_offset=0,
            chunks=[ChunkWitness(_CHUNK_HASH, is_calldata=False, calldata_length=1)],
            opaque_prefix_bytes=b"",
            opaque_suffix_bytes=b"",
            parent_data_rolling_hash=_PARENT_HASH,
            boundary_prev_data_rolling_hash=None,
            segment_end_offsets=[BLOB_BYTES_LENGTH],
        )


def test_calldata_chunk_rejects_zero_length() -> None:
    with pytest.raises(Exception, match="positive calldataLength"):
        _fold_calldata(b"segment", ChunkWitness(_CHUNK_HASH, is_calldata=True, calldata_length=0), [7])


def test_calldata_chunk_rejects_end_past_stream() -> None:
    own_bytes = b"segment"
    chunk = ChunkWitness(Hash32(keccak256(own_bytes)), is_calldata=True, calldata_length=len(own_bytes) + 1)
    with pytest.raises(Exception, match="exceeds the reconstructed stream length"):
        _fold_calldata(own_bytes, chunk, [len(own_bytes)])


def test_calldata_chunk_rejects_end_between_segments() -> None:
    own_bytes = b"firstsecond"
    chunk = ChunkWitness(Hash32(keccak256(own_bytes[:6])), is_calldata=True, calldata_length=6)
    with pytest.raises(Exception, match="does not end at a segment boundary"):
        _fold_calldata(own_bytes, chunk, [5, len(own_bytes)])


def test_calldata_chunk_rejects_wrong_hash() -> None:
    own_bytes = b"segment"
    chunk = ChunkWitness(_CHUNK_HASH, is_calldata=True, calldata_length=len(own_bytes))
    with pytest.raises(Exception, match="computed hash does not match chunkHash"):
        _fold_calldata(own_bytes, chunk, [len(own_bytes)])


def test_calldata_chunk_rejects_start_between_segments(monkeypatch) -> None:
    monkeypatch.setattr(rollup, "_trusted_setup", lambda: object())
    monkeypatch.setattr(rollup.ckzg, "blob_to_kzg_commitment", lambda blob, setup: bytes(48))
    monkeypatch.setattr(rollup, "kzg_commitment_to_versioned_hash", lambda commitment: _CHUNK_HASH)
    calldata = b"segment"
    own_bytes = bytes(BLOB_BYTES_LENGTH) + calldata
    with pytest.raises(Exception, match="does not start at a segment boundary"):
        _verify_and_fold_chunks(
            own_stream_bytes=own_bytes,
            start_offset=0,
            chunks=[
                ChunkWitness(_CHUNK_HASH, is_calldata=False, calldata_length=0),
                ChunkWitness(Hash32(keccak256(calldata)), is_calldata=True, calldata_length=len(calldata)),
            ],
            opaque_prefix_bytes=b"",
            opaque_suffix_bytes=b"",
            parent_data_rolling_hash=_PARENT_HASH,
            boundary_prev_data_rolling_hash=None,
            segment_end_offsets=[len(own_bytes)],
        )


def test_completely_consumed_terminal_blob_uses_canonical_boundary_offset(monkeypatch) -> None:
    assert _fold_blob(monkeypatch, bytes(BLOB_BYTES_LENGTH), b"") == 0


def test_partially_consumed_terminal_blob_uses_consumed_byte_offset(monkeypatch) -> None:
    suffix = bytes(17)
    assert _fold_blob(monkeypatch, bytes(BLOB_BYTES_LENGTH - len(suffix)), suffix) == BLOB_BYTES_LENGTH - len(suffix)


def test_terminal_blob_must_contain_owned_bytes(monkeypatch) -> None:
    monkeypatch.setattr(rollup, "_trusted_setup", lambda: object())

    with pytest.raises(Exception, match="must contain owned bytes"):
        _fold_blob(monkeypatch, b"", bytes(BLOB_BYTES_LENGTH))
