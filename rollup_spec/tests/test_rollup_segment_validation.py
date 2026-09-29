"""The DA parser accepts exactly one complete zstd frame per conflation."""

import pytest
import zstandard as zstd

from rollup_spec.rollup import _validate_conflation_segment


def _frame(payload):
    return zstd.ZstdCompressor().compress(payload)


def test_matching_frame_is_accepted():
    _validate_conflation_segment(_frame(b"canonical payload"), b"canonical payload")


@pytest.mark.parametrize("frame", [b"bad", _frame(b"payload") + b"trailing", _frame(b"payload") + _frame(b"other"), _frame(b"payload")[:-1]])
def test_incomplete_or_extra_frame_is_rejected(frame):
    with pytest.raises(Exception, match="invalid zstd"):
        _validate_conflation_segment(frame, b"payload")


def test_different_canonical_payload_is_rejected():
    with pytest.raises(Exception, match="does not match"):
        _validate_conflation_segment(_frame(b"other"), b"payload")
