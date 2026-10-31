"""Rollup guest over real signed blocks: canonical payload derivation and an end-to-end rollup."""

"""Rollup guest over real signed blocks: canonical payload derivation and an end-to-end rollup."""

from dataclasses import dataclass
from typing import Sequence, Tuple

import pytest
from ethereum.crypto.hash import Hash32, keccak256
from ethereum.state import Address
from ethereum_rlp import rlp
from ethereum_types.numeric import U64

from rollup_spec.l2_execution import (
    L2ExecutionProof,
    L2ExecutionProofPublicInput,
    VerifiableL2ExecutionProof,
    hash_address_list,
    merge_filtered_addresses,
)
from rollup_spec.rollup import (
    ConflationWitness,
    RollupProofPrivateInput,
    derive_canonical_payload,
    rlp_encode_truncated_blocks,
    run_rollup_guest,
    truncate_block_rlp,
)
from tests.blocks import CHAIN_ID, SENDER, ZERO, block, legacy_tx, sign, wire
from tests.da_witness import blob_chunk, calldata_chunk, data_rolling_hash_after, segment

FIRST_PARENT_HASH = keccak256(b"parent of the first block")
PARENT_DATA_ROLLING_HASH = keccak256(b"parent dataRollingHash")


@dataclass(frozen=True)
class SignedBlock:
    block_rlp: bytes
    hash: Hash32
    parent_hash: Hash32


def signed_block(number: int, parent_hash: Hash32) -> SignedBlock:
    """An Amsterdam block holding one EIP-155 legacy transaction sent by `SENDER`."""
    full_block = block([wire(sign(legacy_tx(number)))], number, parent_hash)
    return SignedBlock(rlp.encode(full_block), keccak256(rlp.encode(full_block.header)), parent_hash)


def l2_proof(
    start: int,
    end: int,
    parent_block_hash: Hash32 = ZERO,
    end_block_hash: Hash32 = ZERO,
    senders: Sequence[Address] = (SENDER,),
    bridge_rolling_hash: Tuple[Hash32, U64] = (ZERO, U64(0)),
) -> L2ExecutionProof:
    """
    An l2-execution proof over blocks `start..end`, chaining from and to the given block hashes,
    ending at the given `(rolling hash, message number)` L1->L2 pair.
    """
    public_inputs = L2ExecutionProofPublicInput(
        parent_block_hash=parent_block_hash, end_block_hash=end_block_hash,
        end_block_number=U64(end), end_block_timestamp=U64(end), l2_l1_messages=[],
        end_l1_l2_bridge_rolling_hash=bridge_rolling_hash[0],
        end_l1_l2_bridge_rolling_hash_message_number=bridge_rolling_hash[1],
        dynamic_chain_config_hash=ZERO, parent_ftx_rolling_hash=ZERO, parent_ftx_number=U64(0),
        end_ftx_rolling_hash=ZERO, end_processed_ftx_number=U64(0),
        filtered_addresses=[], tx_froms_hash=hash_address_list(list(senders)),
        block_count=end - start + 1,
    )
    return L2ExecutionProof(public_inputs, U64(start))


def test_derived_conflation_lists_block_hash_and_parent_hash():
    block = signed_block(1, FIRST_PARENT_HASH)

    derived = derive_canonical_payload(ConflationWitness([block.block_rlp]), l2_proof(1, 1), CHAIN_ID)

    assert derived.block_hashes == [block.hash]
    assert derived.parent_hashes == [FIRST_PARENT_HASH]


def test_merge_filtered_addresses_is_distinct_and_byte_ascending():
    low, mid, high = (Address(bytes([n]) * 20) for n in (0x01, 0x02, 0xFF))
    assert merge_filtered_addresses([[high, low, low], [], [mid, high], [low]]) == [low, mid, high]
    assert merge_filtered_addresses([[], []]) == []


def test_tx_froms_hash_omitting_the_block_sender_is_rejected():
    block = signed_block(1, FIRST_PARENT_HASH)

    with pytest.raises(Exception, match="txFromsHash does not match DA block senders"):
        derive_canonical_payload(
            ConflationWitness([block.block_rlp]), l2_proof(1, 1, senders=[]), CHAIN_ID,
        )


def test_block_count_inconsistent_with_the_proof_range_is_rejected():
    block = signed_block(1, FIRST_PARENT_HASH)

    with pytest.raises(Exception, match="block count is inconsistent"):
        derive_canonical_payload(ConflationWitness([block.block_rlp]), l2_proof(1, 2), CHAIN_ID)


def sequencer_payload(block: SignedBlock) -> bytes:
    """The truncated-block RLP the sequencer publishes for a one-block conflation."""
    return rlp_encode_truncated_blocks([truncate_block_rlp(block.block_rlp, CHAIN_ID)])


def two_conflation_rollup_input(
    first_bridge_rolling_hash: Tuple[Hash32, U64] = (ZERO, U64(0)),
    second_bridge_rolling_hash: Tuple[Hash32, U64] = (ZERO, U64(0)),
) -> Tuple[RollupProofPrivateInput, SignedBlock, SignedBlock]:
    """A rollup request over two one-block conflations split across a short blob and calldata."""
    block_1 = signed_block(1, FIRST_PARENT_HASH)
    block_2 = signed_block(2, block_1.hash)
    segment_1, segment_2 = segment(sequencer_payload(block_1)), segment(sequencer_payload(block_2))
    blob = blob_chunk(segment_1 + segment_2[:7])
    calldata = calldata_chunk(segment_2[7:])
    rollup_input = RollupProofPrivateInput(
        parent_data_rolling_hash=PARENT_DATA_ROLLING_HASH,
        parent_data_tail_take_bytes=0,
        chain_id=CHAIN_ID,
        conflations=[ConflationWitness([block_1.block_rlp]), ConflationWitness([block_2.block_rlp])],
        chunks=[blob, calldata],
        l2_execution_proofs=[
            VerifiableL2ExecutionProof(
                l2_proof(1, 1, FIRST_PARENT_HASH, block_1.hash, bridge_rolling_hash=first_bridge_rolling_hash),
                program_vk=ZERO,
            ),
            VerifiableL2ExecutionProof(
                l2_proof(2, 2, block_1.hash, block_2.hash, bridge_rolling_hash=second_bridge_rolling_hash),
                program_vk=ZERO,
            ),
        ],
    )
    return rollup_input, block_1, block_2


def test_rollup_over_two_conflations_split_across_a_short_blob_and_calldata():
    rollup_input, _, block_2 = two_conflation_rollup_input()
    blob, calldata = rollup_input.chunks

    public_inputs = run_rollup_guest(rollup_input).public_inputs

    assert public_inputs.parent_block_hash == FIRST_PARENT_HASH
    assert public_inputs.end_block_hash == block_2.hash
    assert public_inputs.end_data_rolling_hash == data_rolling_hash_after(PARENT_DATA_ROLLING_HASH, [blob, calldata])
    assert public_inputs.final_data_tail_discard_bytes == 0
    assert public_inputs.block_count == 2


FIRST_ROLLING_HASH = (keccak256(b"first rolling hash"), U64(4))
SECOND_ROLLING_HASH = (keccak256(b"second rolling hash"), U64(9))


def test_rollup_without_any_anchored_message_ends_at_the_zero_rolling_hash_pair():
    rollup_input, _, _ = two_conflation_rollup_input()

    public_inputs = run_rollup_guest(rollup_input).public_inputs

    assert public_inputs.end_l1_l2_bridge_rolling_hash == ZERO
    assert public_inputs.end_l1_l2_bridge_rolling_hash_message_number == U64(0)


def test_rollup_ends_at_the_last_conflation_that_anchored_messages():
    rollup_input, _, _ = two_conflation_rollup_input(FIRST_ROLLING_HASH, SECOND_ROLLING_HASH)

    public_inputs = run_rollup_guest(rollup_input).public_inputs

    assert public_inputs.end_l1_l2_bridge_rolling_hash == SECOND_ROLLING_HASH[0]
    assert public_inputs.end_l1_l2_bridge_rolling_hash_message_number == SECOND_ROLLING_HASH[1]


def test_rollup_keeps_the_rolling_hash_pair_when_trailing_conflations_anchor_nothing():
    rollup_input, _, _ = two_conflation_rollup_input(FIRST_ROLLING_HASH, (ZERO, U64(0)))

    public_inputs = run_rollup_guest(rollup_input).public_inputs

    assert public_inputs.end_l1_l2_bridge_rolling_hash == FIRST_ROLLING_HASH[0]
    assert public_inputs.end_l1_l2_bridge_rolling_hash_message_number == FIRST_ROLLING_HASH[1]
