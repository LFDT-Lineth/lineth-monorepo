"""Builders for DA chunk witnesses, anchored the way L1 anchors them."""

from typing import Iterable

import ckzg
import zstandard as zstd
from ethereum.crypto.hash import Hash32, keccak256
from ethereum.crypto.kzg import KZGCommitment, kzg_commitment_to_versioned_hash

from rollup_spec.rollup import ChunkWitness, pack_blob_payload, trusted_setup, zstd_dictionary


def compress_frame(payload: bytes, dictionary: zstd.ZstdCompressionDict | None = None, **parameters) -> bytes:
    """One zstd frame of `payload`, compressed against the program's dictionary unless another is given."""
    dictionary = zstd_dictionary() if dictionary is None else dictionary
    return zstd.ZstdCompressor(dict_data=dictionary, **parameters).compress(payload)


def compress_frame_in_blocks(payload: bytes, window_log: int, block_len: int) -> bytes:
    """One zstd frame of `payload` without content size, under a window of 2**window_log bytes, one block per `block_len` bytes."""
    parameters = zstd.ZstdCompressionParameters.from_level(3, window_log=window_log, write_content_size=False)
    compressor = zstd.ZstdCompressor(dict_data=zstd_dictionary(), compression_params=parameters).compressobj()
    frame = b""
    for start in range(0, len(payload), block_len):
        frame += compressor.compress(payload[start:start + block_len]) + compressor.flush(zstd.COMPRESSOBJ_FLUSH_BLOCK)
    return frame + compressor.flush()


def segment(payload: bytes) -> bytes:
    """One DA stream segment: a 4-byte big-endian length followed by one zstd frame of `payload`."""
    zstd_frame = compress_frame(payload)
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
