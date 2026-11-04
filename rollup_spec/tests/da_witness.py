"""Builders for DA chunk witnesses, anchored the way L1 anchors them."""

from typing import Iterable

import ckzg
import zstandard as zstd
from ethereum.crypto.hash import Hash32, keccak256
from ethereum.crypto.kzg import KZGCommitment, kzg_commitment_to_versioned_hash

from rollup_spec.rollup import ChunkWitness, pack_blob_payload, trusted_setup


def segment(payload: bytes) -> bytes:
    """One DA stream segment: a 4-byte big-endian length followed by one zstd frame of `payload`."""
    zstd_frame = zstd.ZstdCompressor().compress(payload)
    return len(zstd_frame).to_bytes(4, "big") + zstd_frame


def blob_chunk(payload: bytes) -> ChunkWitness:
    """A blob chunk anchored by the versioned hash of its commitment, carrying the EIP-4844 blob proof."""
    blob = pack_blob_payload(payload)
    commitment = ckzg.blob_to_kzg_commitment(blob, trusted_setup())
    proof = ckzg.compute_blob_kzg_proof(blob, commitment, trusted_setup())
    return ChunkWitness(
        kzg_commitment_to_versioned_hash(KZGCommitment(commitment)),
        is_calldata=False,
        blob_bytes=blob,
        kzg_commitment=commitment,
        kzg_proof=proof,
    )


def calldata_chunk(data: bytes) -> ChunkWitness:
    return ChunkWitness(keccak256(data), is_calldata=True, calldata_bytes=data)


def data_rolling_hash_after(parent: Hash32, chunks: Iterable[ChunkWitness]) -> Hash32:
    """The dataRollingHash after folding `chunks` onto `parent` (§3.1)."""
    for chunk in chunks:
        parent = keccak256(parent + chunk.chunk_hash)
    return parent
