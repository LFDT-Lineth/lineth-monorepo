from typing import List, Sequence


def rebase_messaging_offsets(
    start_block_number: int,
    end_block_number: int,
    block_count: int,
    offsets: Sequence[int],
    range_start_block_number: int,
    proof_kind: str,
    output_kind: str,
) -> List[int]:
    """Validate a proof's block-relative offsets and rebase them into a larger range."""
    count = end_block_number - start_block_number + 1
    if count <= 0 or block_count != count:
        raise Exception(f"{proof_kind} blockCount does not match proven range")

    rebased_offsets: List[int] = []
    previous = 0
    for offset in offsets:
        if type(offset) is not int or not previous < offset <= count or offset > 0xFFFF:
            raise Exception(f"invalid {proof_kind} messaging block offset")
        rebased = start_block_number - range_start_block_number + offset
        if rebased > 0xFFFF:
            raise Exception(f"{output_kind} messaging block offset exceeds uint16")
        rebased_offsets.append(rebased)
        previous = offset
    return rebased_offsets
