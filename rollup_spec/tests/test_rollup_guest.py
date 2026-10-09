"""Rollup guest over real signed blocks: canonical payload derivation and an end-to-end rollup."""

from dataclasses import dataclass, replace
from typing import Sequence

import pytest
from coincurve import PrivateKey
from ethereum.crypto.hash import Hash32, keccak256
from ethereum.forks.amsterdam.transactions import signing_hash_155
from ethereum.state import Address
from ethereum_rlp import rlp
from ethereum_types.bytes import Bytes8, Bytes20, Bytes32, Bytes256
from ethereum_types.numeric import U64, U256, Uint

from rollup_spec.fork import Block, Header, LegacyTransaction
from rollup_spec.l2_execution import (
    L2ExecutionProof,
    L2ExecutionProofPublicInput,
    VerifiableL2ExecutionProof,
    hash_address_list,
)
from rollup_spec.rollup import (
    ConflationWitness,
    RollupProofPrivateInput,
    derive_canonical_payload,
    rlp_encode_truncated_blocks,
    run_rollup_guest,
    truncate_block_rlp,
)
from tests.da_witness import blob_chunk, calldata_chunk, data_rolling_hash_after, segment

CHAIN_ID = U64(59144)
SECRET_KEY = bytes([0x42]) * 32
SENDER = Address(keccak256(PrivateKey(SECRET_KEY).public_key.format(compressed=False)[1:])[12:])

ZERO = Hash32(bytes(32))
FIRST_PARENT_HASH = keccak256(b"parent of the first block")
PARENT_DATA_ROLLING_HASH = keccak256(b"parent dataRollingHash")


@dataclass(frozen=True)
class SignedBlock:
    block_rlp: bytes
    hash: Hash32
    parent_hash: Hash32


def signed_block(number: int, parent_hash: Hash32) -> SignedBlock:
    """An Amsterdam block holding one EIP-155 legacy transaction sent by `SENDER`."""
    unsigned = LegacyTransaction(
        nonce=U256(number), gas_price=Uint(7), gas=Uint(21_000), to=Bytes20(b"\x11" * 20),
        value=U256(1), data=b"", v=U256(0), r=U256(0), s=U256(0),
    )
    signature = PrivateKey(SECRET_KEY).sign_recoverable(signing_hash_155(unsigned, CHAIN_ID), hasher=None)
    tx = replace(
        unsigned,
        v=U256(signature[64] + 35 + 2 * int(CHAIN_ID)),
        r=U256(int.from_bytes(signature[:32], "big")),
        s=U256(int.from_bytes(signature[32:64], "big")),
    )
    root = Bytes32(bytes(32))
    header = Header(
        parent_hash=parent_hash, ommers_hash=root, coinbase=Bytes20(bytes(20)), state_root=root,
        transactions_root=root, receipt_root=root, bloom=Bytes256(bytes(256)), difficulty=Uint(0),
        number=Uint(number), gas_limit=Uint(30_000_000), gas_used=Uint(21_000),
        timestamp=U256(1_700_000_000 + number), extra_data=b"", prev_randao=root, nonce=Bytes8(bytes(8)),
        base_fee_per_gas=Uint(7), withdrawals_root=root, blob_gas_used=U64(0), excess_blob_gas=U64(0),
        parent_beacon_block_root=root, requests_hash=root, block_access_list_hash=root,
        slot_number=U64(number),
    )
    block = Block(header=header, transactions=(tx,), ommers=(), withdrawals=())
    return SignedBlock(rlp.encode(block), keccak256(rlp.encode(header)), parent_hash)


def l2_proof(
    start: int,
    end: int,
    parent_block_hash: Hash32 = ZERO,
    end_block_hash: Hash32 = ZERO,
    senders: Sequence[Address] = (SENDER,),
) -> L2ExecutionProof:
    """An l2-execution proof over blocks `start..end`, chaining from and to the given block hashes."""
    public_inputs = L2ExecutionProofPublicInput(
        parent_block_hash=parent_block_hash, end_block_hash=end_block_hash,
        end_block_number=U64(end), end_block_timestamp=U64(end), l2_l1_messages=[],
        parent_l1_l2_bridge_rolling_hash=ZERO, parent_l1_l2_bridge_rolling_hash_message_number=U64(0),
        end_l1_l2_bridge_rolling_hash=ZERO, end_l1_l2_bridge_rolling_hash_message_number=U64(0),
        dynamic_chain_config_hash=ZERO, parent_ftx_rolling_hash=ZERO, parent_ftx_number=U64(0),
        end_ftx_rolling_hash=ZERO, end_processed_ftx_number=U64(0),
        filtered_addresses_hash=hash_address_list([]), tx_froms_hash=hash_address_list(list(senders)),
        block_count=end - start + 1,
    )
    return L2ExecutionProof(public_inputs, U64(start))


def test_derived_conflation_lists_block_hash_and_parent_hash():
    block = signed_block(1, FIRST_PARENT_HASH)

    derived = derive_canonical_payload(ConflationWitness([block.block_rlp]), l2_proof(1, 1), CHAIN_ID)

    assert derived.block_hashes == [block.hash]
    assert derived.parent_hashes == [FIRST_PARENT_HASH]


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


def test_rollup_over_two_conflations_split_across_a_short_blob_and_calldata():
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
            VerifiableL2ExecutionProof(l2_proof(1, 1, FIRST_PARENT_HASH, block_1.hash), program_vk=ZERO),
            VerifiableL2ExecutionProof(l2_proof(2, 2, block_1.hash, block_2.hash), program_vk=ZERO),
        ],
    )

    public_inputs = run_rollup_guest(rollup_input).public_inputs

    assert public_inputs.parent_block_hash == FIRST_PARENT_HASH
    assert public_inputs.end_block_hash == block_2.hash
    assert public_inputs.end_data_rolling_hash == data_rolling_hash_after(PARENT_DATA_ROLLING_HASH, [blob, calldata])
    assert public_inputs.final_data_tail_discard_bytes == 0
    assert public_inputs.block_count == 2
