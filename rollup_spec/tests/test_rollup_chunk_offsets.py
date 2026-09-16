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
        chunks=[ChunkWitness(_CHUNK_HASH)],
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
        chunks=[ChunkWitness(Hash32(keccak256(own_bytes)), is_calldata=True)],
        opaque_prefix_bytes=b"",
        opaque_suffix_bytes=b"",
        parent_data_rolling_hash=_PARENT_HASH,
        boundary_prev_data_rolling_hash=None,
        segment_end_offsets=[len(own_bytes)],
    )

    assert end_offset == 0


def test_completely_consumed_terminal_blob_uses_canonical_boundary_offset(monkeypatch) -> None:
    assert _fold_blob(monkeypatch, bytes(BLOB_BYTES_LENGTH), b"") == 0


def test_partially_consumed_terminal_blob_uses_consumed_byte_offset(monkeypatch) -> None:
    suffix = bytes(17)
    assert _fold_blob(monkeypatch, bytes(BLOB_BYTES_LENGTH - len(suffix)), suffix) == BLOB_BYTES_LENGTH - len(suffix)


def test_terminal_blob_must_contain_owned_bytes(monkeypatch) -> None:
    monkeypatch.setattr(rollup, "_trusted_setup", lambda: object())

    with pytest.raises(Exception, match="must contain owned bytes"):
        _fold_blob(monkeypatch, b"", bytes(BLOB_BYTES_LENGTH))
