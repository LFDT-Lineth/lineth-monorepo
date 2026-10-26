"""Builders for DA chunk witnesses, anchored the way L1 anchors them."""

from typing import Iterable

import zstandard as zstd
from ethereum.crypto.hash import Hash32, keccak256

from rollup_spec.rollup import ChunkWitness, blob_versioned_hash, pack_blob_payload


def segment(payload: bytes) -> bytes:
    """One DA stream segment: a 4-byte big-endian length followed by one zstd frame of `payload`."""
    zstd_frame = zstd.ZstdCompressor().compress(payload)
    return len(zstd_frame).to_bytes(4, "big") + zstd_frame


def blob_chunk(payload: bytes) -> ChunkWitness:
    blob = pack_blob_payload(payload)
    return ChunkWitness(blob_versioned_hash(blob), is_calldata=False, blob_bytes=blob)


def calldata_chunk(data: bytes) -> ChunkWitness:
    return ChunkWitness(keccak256(data), is_calldata=True, calldata_bytes=data)


def data_rolling_hash_after(parent: Hash32, chunks: Iterable[ChunkWitness]) -> Hash32:
    """The dataRollingHash after folding `chunks` onto `parent` (§3.1)."""
    for chunk in chunks:
        parent = keccak256(parent + chunk.chunk_hash)
    return parent
