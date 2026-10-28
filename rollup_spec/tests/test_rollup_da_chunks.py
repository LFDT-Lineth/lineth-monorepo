"""DA stream verification: segment ownership across blob and calldata chunks, chunk binding, segment validity."""

from dataclasses import replace

import pytest
import zstandard as zstd
from ethereum.crypto.hash import keccak256

from rollup_spec.rollup import BLOB_PAYLOAD_CAPACITY, ChunkWitness, verify_da_chunks
from tests.da_witness import blob_chunk, calldata_chunk, data_rolling_hash_after, segment

PAYLOAD_1 = b"conflation 1"
PAYLOAD_2 = b"conflation 2"
SEGMENT_1 = segment(PAYLOAD_1)
SEGMENT_2 = segment(PAYLOAD_2)
ZSTD_FRAME_1 = zstd.ZstdCompressor().compress(PAYLOAD_1)

PARENT = keccak256(b"parent dataRollingHash")
BEFORE_FIRST_CHUNK = keccak256(b"dataRollingHash before the first touched chunk")
WRONG_HASH = keccak256(b"unrelated hash")

FOREIGN_PREFIX = b"segments owned by the previous proof"
FOREIGN_SUFFIX = b"segments owned by the next proof"

INSIDE_LENGTH_PREFIX, INSIDE_BODY = 2, 7  # offsets into a segment, whose first 4 bytes are its length prefix
SPLITS = pytest.mark.parametrize(
    "split", [INSIDE_LENGTH_PREFIX, INSIDE_BODY], ids=["inside length prefix", "inside body"],
)


def length_prefixed(zstd_frame: bytes) -> bytes:
    return len(zstd_frame).to_bytes(4, "big") + zstd_frame


def verify_from_mid_blob(chunks, payloads, take):
    """Verify a stream that starts in the last `take` bytes of its first blob, which the parent hash folds."""
    parent = data_rolling_hash_after(BEFORE_FIRST_CHUNK, chunks[:1])
    return verify_da_chunks(chunks, payloads, parent, take, BEFORE_FIRST_CHUNK)


# ---- accepted layouts ------------------------------------------------------------------------


def test_blob_holding_one_segment():
    blob = blob_chunk(SEGMENT_1)

    assert verify_da_chunks([blob], [PAYLOAD_1], PARENT) == (data_rolling_hash_after(PARENT, [blob]), 0)


def test_shared_blob_with_foreign_prefix_and_suffix():
    blob = blob_chunk(FOREIGN_PREFIX + SEGMENT_1 + FOREIGN_SUFFIX)
    take = len(SEGMENT_1) + len(FOREIGN_SUFFIX)

    end_hash, discard = verify_from_mid_blob([blob], [PAYLOAD_1], take)

    # The parent hash already folds the shared blob, so nothing is folded again.
    assert (end_hash, discard) == (data_rolling_hash_after(BEFORE_FIRST_CHUNK, [blob]), len(FOREIGN_SUFFIX))


@SPLITS
def test_segment_crossing_full_blob_into_next_blob(split):
    first = blob_chunk(bytes(BLOB_PAYLOAD_CAPACITY - split) + SEGMENT_1[:split])
    second = blob_chunk(SEGMENT_1[split:])

    result = verify_from_mid_blob([first, second], [PAYLOAD_1], take=split)

    assert result == (data_rolling_hash_after(BEFORE_FIRST_CHUNK, [first, second]), 0)


@SPLITS
def test_segment_crossing_short_blob_into_calldata(split):
    blob = blob_chunk(SEGMENT_1[:split])
    calldata = calldata_chunk(SEGMENT_1[split:])

    result = verify_da_chunks([blob, calldata], [PAYLOAD_1], PARENT)

    assert result == (data_rolling_hash_after(PARENT, [blob, calldata]), 0)


def test_calldata_finishing_the_blob_segment_and_carrying_a_second_segment():
    blob = blob_chunk(SEGMENT_1[:INSIDE_BODY])
    calldata = calldata_chunk(SEGMENT_1[INSIDE_BODY:] + SEGMENT_2)

    result = verify_da_chunks([blob, calldata], [PAYLOAD_1, PAYLOAD_2], PARENT)

    assert result == (data_rolling_hash_after(PARENT, [blob, calldata]), 0)


def test_calldata_holding_a_whole_segment_after_a_short_blob():
    blob = blob_chunk(SEGMENT_1)
    calldata = calldata_chunk(SEGMENT_2)

    result = verify_da_chunks([blob, calldata], [PAYLOAD_1, PAYLOAD_2], PARENT)

    assert result == (data_rolling_hash_after(PARENT, [blob, calldata]), 0)


def test_shared_blob_followed_by_calldata_finishing_the_segment():
    blob = blob_chunk(FOREIGN_PREFIX + SEGMENT_1[:INSIDE_LENGTH_PREFIX])
    calldata = calldata_chunk(SEGMENT_1[INSIDE_LENGTH_PREFIX:])

    result = verify_from_mid_blob([blob, calldata], [PAYLOAD_1], take=INSIDE_LENGTH_PREFIX)

    assert result == (data_rolling_hash_after(BEFORE_FIRST_CHUNK, [blob, calldata]), 0)


def test_calldata_followed_by_blob():
    calldata = calldata_chunk(SEGMENT_1)
    blob = blob_chunk(SEGMENT_2)

    result = verify_da_chunks([calldata, blob], [PAYLOAD_1, PAYLOAD_2], PARENT)

    assert result == (data_rolling_hash_after(PARENT, [calldata, blob]), 0)


@pytest.mark.parametrize("write_content_size", [True, False], ids=["content size written", "content size omitted"])
def test_zstd_frame_content_size_header_is_optional(write_content_size):
    zstd_frame = zstd.ZstdCompressor(write_content_size=write_content_size).compress(PAYLOAD_1)
    calldata = calldata_chunk(length_prefixed(zstd_frame))

    assert verify_da_chunks([calldata], [PAYLOAD_1], PARENT) == (data_rolling_hash_after(PARENT, [calldata]), 0)


# ---- rejected streams ------------------------------------------------------------------------


def rejected(case_id, error, build_chunks, payloads=(PAYLOAD_1,), take=0):
    """A rejected stream; chunks are built lazily so collecting the table does no KZG work."""
    return pytest.param(build_chunks, list(payloads), take, error, id=case_id)


def blob_missing_last_byte() -> ChunkWitness:
    blob = blob_chunk(SEGMENT_1)
    return replace(blob, blob_bytes=blob.blob_bytes[:-1])


REJECTED_STREAMS = [
    # calldata must end on a segment boundary
    rejected(
        "calldata ends mid-segment before calldata", "segment boundary",
        lambda: [calldata_chunk(SEGMENT_1[:5]), calldata_chunk(SEGMENT_1[5:])],
    ),
    rejected(
        "calldata ends mid-segment before blob", "segment boundary",
        lambda: [calldata_chunk(SEGMENT_1[:5]), blob_chunk(SEGMENT_1[5:])],
    ),
    rejected(
        "calldata ends mid-segment after blob", "segment boundary",
        lambda: [blob_chunk(SEGMENT_1[:5]), calldata_chunk(SEGMENT_1[5:10]), calldata_chunk(SEGMENT_1[10:])],
    ),
    rejected(
        "terminal calldata has trailing bytes", "segment boundary",
        lambda: [calldata_chunk(SEGMENT_1 + b"extra")],
    ),
    rejected(
        "terminal calldata has a partial extra segment", "segment boundary",
        lambda: [calldata_chunk(SEGMENT_1 + SEGMENT_2[:5])],
    ),
    # blob before blob must be full
    rejected(
        "short blob before blob", "full payload",
        lambda: [blob_chunk(SEGMENT_1), blob_chunk(SEGMENT_2)], payloads=(PAYLOAD_1, PAYLOAD_2),
    ),
    # every chunk owns at least one byte
    rejected(
        "blob after the last segment", "must contain owned bytes",
        lambda: [blob_chunk(SEGMENT_1 + bytes(BLOB_PAYLOAD_CAPACITY - len(SEGMENT_1))), blob_chunk(SEGMENT_2)],
    ),
    rejected(
        "empty blob between calldata", "must contain owned bytes",
        lambda: [calldata_chunk(SEGMENT_1), blob_chunk(b""), calldata_chunk(SEGMENT_2)],
        payloads=(PAYLOAD_1, PAYLOAD_2),
    ),
    # a positive take starts inside a first blob with a longer payload
    rejected(
        "positive take with calldata first", "requires a first blob with a longer payload",
        lambda: [calldata_chunk(SEGMENT_1)], take=3,
    ),
    rejected(
        "take equals the first blob payload length", "requires a first blob with a longer payload",
        lambda: [blob_chunk(SEGMENT_1)], take=len(SEGMENT_1),
    ),
    rejected(
        "take exceeds the first blob payload length", "requires a first blob with a longer payload",
        lambda: [blob_chunk(SEGMENT_1)], take=len(SEGMENT_1) + 1,
    ),
    # chunk bytes bind to the anchored hash
    rejected("invalid physical blob", "physical blob", lambda: [blob_missing_last_byte()]),
    rejected(
        "blob with wrong chunk hash", "KZG commitment does not match chunkHash",
        lambda: [replace(blob_chunk(SEGMENT_1), chunk_hash=WRONG_HASH)],
    ),
    rejected(
        "calldata with wrong chunk hash", "computed hash does not match chunkHash",
        lambda: [replace(calldata_chunk(SEGMENT_1), chunk_hash=WRONG_HASH)],
    ),
    rejected(
        "blob carrying calldata bytes", "empty calldataBytes",
        lambda: [replace(blob_chunk(SEGMENT_1), calldata_bytes=SEGMENT_1)],
    ),
    rejected(
        "calldata carrying blob bytes", "empty blobBytes",
        lambda: [replace(calldata_chunk(SEGMENT_1), blob_bytes=blob_chunk(SEGMENT_1).blob_bytes)],
    ),
    # each segment is a length prefix and one zstd frame that decompresses to its canonical payload
    rejected(
        "missing length prefix", "missing its length prefix",
        lambda: [calldata_chunk(b"\x00\x01")],
    ),
    rejected(
        "zero length", "length exceeds available chunk bytes",
        lambda: [calldata_chunk(length_prefixed(b""))],
    ),
    rejected(
        "length past the stream", "length exceeds available chunk bytes",
        lambda: [calldata_chunk((len(ZSTD_FRAME_1) + 1).to_bytes(4, "big") + ZSTD_FRAME_1)],
    ),
    rejected(
        "invalid zstd", "invalid zstd",
        lambda: [calldata_chunk(length_prefixed(b"not a zstd frame"))],
    ),
    rejected(
        "trailing data after the zstd frame", "invalid zstd",
        lambda: [calldata_chunk(length_prefixed(ZSTD_FRAME_1 + b"trailing"))],
    ),
    rejected(
        "two zstd frames under one length", "invalid zstd",
        lambda: [calldata_chunk(length_prefixed(ZSTD_FRAME_1 + zstd.ZstdCompressor().compress(PAYLOAD_2)))],
    ),
    rejected(
        "truncated zstd frame", "invalid zstd",
        lambda: [calldata_chunk(length_prefixed(ZSTD_FRAME_1[:-1]))],
    ),
    rejected(
        "decompressed bytes differ from the canonical payload", "does not match canonical",
        lambda: [calldata_chunk(length_prefixed(zstd.ZstdCompressor().compress(b"conflation X")))],
    ),
]


@pytest.mark.parametrize("build_chunks, payloads, take, error", REJECTED_STREAMS)
def test_invalid_da_stream_is_rejected(build_chunks, payloads, take, error):
    with pytest.raises(Exception, match=error):
        verify_da_chunks(build_chunks(), payloads, PARENT, take)


@pytest.mark.parametrize("boundary", [None, WRONG_HASH], ids=["missing", "wrong"])
def test_mid_blob_start_rejects_boundary_hash_that_does_not_open_the_parent_hash(boundary):
    blob = blob_chunk(FOREIGN_PREFIX + SEGMENT_1)
    parent = data_rolling_hash_after(BEFORE_FIRST_CHUNK, [blob])

    with pytest.raises(Exception, match="boundaryPrevDataRollingHash"):
        verify_da_chunks([blob], [PAYLOAD_1], parent, len(SEGMENT_1), boundary)
