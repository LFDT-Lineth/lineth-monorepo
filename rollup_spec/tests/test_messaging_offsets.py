import pytest

from rollup_spec.messaging_offsets import rebase_messaging_offsets


def test_offsets_are_rebased_from_proof_range_to_aggregation_range() -> None:
    assert rebase_messaging_offsets(101, 110, 10, [1, 10], 91, "rollup", "aggregation") == [11, 20]
    assert rebase_messaging_offsets(101, 110, 10, [], 91, "rollup", "aggregation") == []


@pytest.mark.parametrize("offsets", [[0], [-1], [11], [2, 2], [3, 2], [True], [1.5], [65536]])
def test_offsets_must_be_ordered_uint16_positions_inside_the_proof_range(offsets: list[int]) -> None:
    with pytest.raises(Exception, match="invalid rollup messaging block offset"):
        rebase_messaging_offsets(101, 110, 10, offsets, 91, "rollup", "aggregation")


@pytest.mark.parametrize("start, end, count", [(101, 110, 9), (101, 100, 0)])
def test_block_count_must_match_proven_range(start: int, end: int, count: int) -> None:
    with pytest.raises(Exception, match="rollup blockCount does not match proven range"):
        rebase_messaging_offsets(start, end, count, [1], 91, "rollup", "aggregation")


def test_offset_valid_within_proof_cannot_overflow_aggregation_uint16_range() -> None:
    assert rebase_messaging_offsets(65535, 65535, 1, [1], 1, "rollup", "aggregation") == [65535]
    with pytest.raises(Exception, match="aggregation messaging block offset exceeds uint16"):
        rebase_messaging_offsets(65536, 65536, 1, [1], 1, "rollup", "aggregation")
