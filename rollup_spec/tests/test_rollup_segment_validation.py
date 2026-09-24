"""Unit tests for the rollup guest's witnessed zstd segment validation."""

import pytest
import zstandard as zstd

from rollup_spec.rollup import (
    _validate_conflation_segment,
)


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


def test_validate_conflation_segment_rejects_mismatched_decompressed_bytes() -> None:
    with pytest.raises(Exception, match="does not match"):
        _validate_conflation_segment(_segment(b"other!!"), b"payload")
