"""Unit tests for the rollup guest's witnessed zstd segment validation."""

import pytest
import zstandard as zstd

from rollup_spec.rollup import _validate_conflation_segment


def _segment(payload: bytes) -> bytes:
    return zstd.ZstdCompressor().compress(payload)


@pytest.mark.parametrize("write_content_size", [True, False])
def test_validate_conflation_segment_accepts_matching_zstd_frame(write_content_size: bool) -> None:
    expected = b"canonical truncated-block RLP"
    frame = zstd.ZstdCompressor(write_content_size=write_content_size).compress(expected)

    _validate_conflation_segment(frame, expected)


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
