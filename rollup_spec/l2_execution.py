from dataclasses import dataclass, field
from typing import List, Sequence, Tuple

from ethereum.crypto.hash import Hash32, keccak256
from ethereum.forks.osaka.blocks import Block as EthereumBlock, Header
from ethereum.forks.osaka.transactions import (
    BlobTransaction,
    FeeMarketTransaction,
    SetCodeTransaction,
    recover_sender,
)
from ethereum.forks.osaka.vm.gas import calculate_total_blob_gas
from ethereum.state import Address
from ethereum_types.bytes import Bytes32
from ethereum_types.numeric import U64, Uint

from .block import (
    ChainConfig,
    ForcedTransactionAcceptance,
    ResolvedForcedTransaction,
    RollupBlock,
    block_hash,
    decode_block_rlp,
    decode_signed_transaction_rlp,
    parse_block_transaction_rlps,
    resolve_forced_transaction,
)
from .state_transition import ExecutionWitness, L2State, state_transition_modified

BRIDGE_L2L1_MESSAGE_SENT_TOPIC_0 = Hash32(
    bytes.fromhex("e856c2b8bd4eb0027ce32eeaf595c21b0b6b4644b326e5b7bd80a1cf8db72e6c"),
)

# Storage layout of the L2MessageService contract.
#
# The L1->L2 rolling hash is not a single slot — it lives in the
# `l1RollingHashes` mapping keyed by message number, with the latest message
# number tracked separately in `lastAnchoredL1MessageNumber`. The two slot
# positions below are extracted from the compiled storage layout of
# `contracts/src/messaging/l2/L2MessageService.sol` (build-info JSON in
# `contracts/build/build-info/`; the layout is reproduced via
# `_storage_layout_table` in the script that maintains these constants).
# If the contract's storage layout changes — including adding/removing
# upgrade-safety `__gap` slots in any ancestor — these slot indices MUST
# be re-extracted; otherwise the proof reads zero / unrelated values and
# the L1 finalization check on `l1RollingHash[messageNumber]` will fail.
LAST_ANCHORED_L1_MESSAGE_NUMBER_SLOT: Bytes32 = Bytes32(int(280).to_bytes(32, "big"))
L1_ROLLING_HASHES_MAPPING_BASE_SLOT: Bytes32 = Bytes32(int(281).to_bytes(32, "big"))


def _mapping_slot(base_slot: Bytes32, key: bytes) -> Bytes32:
    """
    Solidity mapping slot computation: slot = keccak256(key || base_slot).
    `key` is the abi-encoded mapping key padded to 32 bytes.
    """
    padded_key = key.rjust(32, b"\x00") if len(key) < 32 else bytes(key)
    return Bytes32(keccak256(padded_key + bytes(base_slot)))


def read_l1l2_bridge_state(state: L2State, l2_message_service_address: Address) -> Tuple[Hash32, U64]:
    """
    Read the L1->L2 bridge rolling hash and its associated message number
    from the L2MessageService contract's storage at `state.state_root`.
    Reads through the EVM state interface — the production guest verifies
    the corresponding MPT proof paths against `state.state_root` from the
    `ExecutionWitness.state` node pool.

    Two reads: (a) `lastAnchoredL1MessageNumber` at a fixed slot, and
    (b) `l1RollingHashes[lastAnchoredL1MessageNumber]` at the mapping slot
    computed via `keccak256(uint256_be(messageNumber) || base_slot)`.

    PRECOMPILE (production guest): keccak256 (mapping-slot computation in
    `_mapping_slot` below) and the MPT-walk hashes inside `state.storage()`.
    """
    number_bytes = state.storage(l2_message_service_address, LAST_ANCHORED_L1_MESSAGE_NUMBER_SLOT)
    rolling_hash_number = U64(int.from_bytes(bytes(number_bytes), "big"))

    rolling_hash_slot = _mapping_slot(
        L1_ROLLING_HASHES_MAPPING_BASE_SLOT,
        int(rolling_hash_number).to_bytes(32, "big"),
    )
    rolling_hash = Hash32(state.storage(l2_message_service_address, rolling_hash_slot))
    return rolling_hash, rolling_hash_number


def add_to_forced_tx_rolling_hash(
    forced_tx_rolling_hash: Hash32,
    ftx: ResolvedForcedTransaction,
) -> Tuple[Hash32, U64]:
    """
    Update the forced-transaction rolling hash with an already-resolved FTX.
    Formula matches §6.3: keccak256(prev || txHash || deadline || from).
    """
    return keccak256(
        forced_tx_rolling_hash +
        ftx.tx_hash +
        int(ftx.deadline).to_bytes(32, "big") +
        bytes(ftx.from_address)
    ), ftx.number


def validate_forced_transactions(
    curr_rolling_hash: Hash32,
    last_processed_ftx_number: U64,
    chain_config: ChainConfig,
    block_header: Header,
    parent_state: L2State,
    block: RollupBlock,
) -> Tuple[List[Address], Hash32, U64]:
    """
    Scan the forced transactions in this block and assert each has the
    correct outcome (Included / Invalid sub-case / Refused sub-case,
    §6.5). For the Invalid sub-cases, the FTX sender's account is read
    from `parent_state` (the L2 state at the parent of this block) via
    the EVM state interface; the witness pool in `ExecutionWitness.state`
    must include that MPT path.

    Dispatch on `resolved_ftx.acceptance`:
      - FILTERED_ADDRESS_FROM | FILTERED_ADDRESS_TO         -> Refused;
        bubble up the relevant address for the L1 sanction-list check.
      - INCLUDED                                            -> assert
        `txHash` is in the block's tx list.
      - BAD_NONCE | BAD_BALANCE                              -> Invalid;
        assert `txHash` is NOT in the block AND that the specific
        pre-validation failure holds against the sender's account read
        from `parent_state`.
    """
    rejected_addresses: List[Address] = []

    for ftx in block.forced_transactions:
        # FTXs are processed in ascending L1-assigned number.
        if ftx.number != last_processed_ftx_number + 1:
            raise Exception("forced transactions must be processed in ascending sequence")

        # Deadline constraint (§6.5): the FTX must be handled in a block whose
        # number does not exceed its declared deadline.
        if ftx.deadline < block_header.number:
            raise Exception("deadline exceeded")

        resolved_ftx = resolve_forced_transaction(ftx, chain_config.chain_id)
        transaction = resolved_ftx.transaction
        from_address = resolved_ftx.from_address

        # The rolling hash is updated for every FTX in the proof range,
        # regardless of outcome.
        curr_rolling_hash, last_processed_ftx_number = add_to_forced_tx_rolling_hash(
            curr_rolling_hash, resolved_ftx,
        )

        # Refused (sanction list) — bubble up the relevant address. The L1
        # contract verifies a-posteriori that each bubbled address appears
        # on its reference sanction list.
        if resolved_ftx.acceptance == ForcedTransactionAcceptance.FILTERED_ADDRESS_FROM:
            rejected_addresses.append(from_address)
            continue

        if resolved_ftx.acceptance == ForcedTransactionAcceptance.FILTERED_ADDRESS_TO:
            # Contract-creation transactions have no recipient (to == None);
            # FILTERED_ADDRESS_TO is meaningless for them.
            if not isinstance(transaction.to, Address):
                raise Exception("FILTERED_ADDRESS_TO on a contract-creation transaction")
            rejected_addresses.append(transaction.to)
            continue

        # Block-membership check: INCLUDED variants must appear in the
        # block; the two Invalid variants must NOT appear.
        block_tx_hashes = [
            keccak256(tx_rlp)
            for tx_rlp in parse_block_transaction_rlps(block.block_rlp)
        ]
        tx_in_block = resolved_ftx.tx_hash in block_tx_hashes

        if resolved_ftx.acceptance == ForcedTransactionAcceptance.INCLUDED:
            if not tx_in_block:
                raise Exception("INCLUDED FTX was not found in the block's transaction list")
            # The EVM state transition proves the FTX executed validly as part
            # of the block; nothing more to check at this layer.
            continue

        if resolved_ftx.acceptance not in (
            ForcedTransactionAcceptance.BAD_NONCE,
            ForcedTransactionAcceptance.BAD_BALANCE,
        ):
            raise Exception("forced transaction has an unknown acceptance value")

        if tx_in_block:
            raise Exception(
                "FTX declared as one of the Invalid sub-cases but was found in the block"
            )

        # Invalid — pre-validation must fail against the L2 state at this
        # block's parent state root. The witness pool must include the MPT
        # path for the sender account.
        sender_account = parent_state.account(from_address)
        if sender_account is None:
            raise Exception("FTX-invalid: sender account absent from L2 state")

        # Dispatch on the specific Invalid sub-case so the spec/PI carries the
        # actual reason the FTX failed.
        if resolved_ftx.acceptance == ForcedTransactionAcceptance.BAD_NONCE:
            if sender_account.nonce == Uint(transaction.nonce):
                raise Exception("BAD_NONCE declared but account.nonce matches tx.nonce")
            continue

        # BAD_BALANCE — the sender's balance must be less than the maximum gas
        # cost plus the transferred value. We mirror the gas-cost arithmetic
        # from `is_valid_forced_transaction` (which itself mirrors
        # `fork.check_transaction`).
        if isinstance(transaction, (FeeMarketTransaction, BlobTransaction, SetCodeTransaction)):
            max_gas_fee = transaction.gas * transaction.max_fee_per_gas
            if isinstance(transaction, BlobTransaction):
                max_gas_fee += Uint(calculate_total_blob_gas(transaction)) * Uint(transaction.max_fee_per_blob_gas)
        else:
            max_gas_fee = transaction.gas * transaction.gas_price

        if Uint(sender_account.balance) >= max_gas_fee + Uint(transaction.value):
            raise Exception("BAD_BALANCE declared but account.balance covers gas+value")

    return rejected_addresses, curr_rolling_hash, last_processed_ftx_number


@dataclass
class L2ExecutionProofPublicInput:
    """
    The 15-field l2-execution public input tuple from Readme.md section 2.1.
    """
    parent_block_hash: Hash32
    end_block_hash: Hash32
    end_block_number: U64
    end_block_timestamp: U64
    l2_l1_messages_hash: Hash32
    parent_l1_l2_bridge_rolling_hash: Hash32
    parent_l1_l2_bridge_rolling_hash_message_number: U64
    end_l1_l2_bridge_rolling_hash: Hash32
    end_l1_l2_bridge_rolling_hash_message_number: U64
    dynamic_chain_config_hash: Hash32
    parent_ftx_rolling_hash: Hash32
    end_ftx_rolling_hash: Hash32
    last_processed_ftx_number: U64
    filtered_addresses_hash: Hash32
    tx_froms_hash: Hash32


@dataclass
class L2ExecutionProofPrivateInput:
    """
    Logical l2-execution request. `blocks` carries canonical block RLP bytes
    plus per-block FTX metadata. `execution_witnesses` carries the corresponding
    Besu `debug_executionWitness` payload for each block. The first witness must
    contain the parent header whose hash is `blocks[0].header.parent_hash`.

    The L1->L2 rolling-hash boundary values are not separate witness fields:
    the guest reads them directly from the L2 state at the parent and end
    state roots via the EVM state interface (`L2State.storage`). The witness
    producer must ensure the relevant MPT paths are in
    `ExecutionWitness.state`
    """
    parent_ftx_rolling_hash: Hash32
    parent_last_processed_ftx_number: U64
    blocks: List[RollupBlock]
    execution_witnesses: List[ExecutionWitness]
    chain_config: ChainConfig


@dataclass
class L2ExecutionProof:
    """
    Reference wrapper for an l2-execution proof plus the hash preimages consumed
    by the rollup guest. `proof` stands in for the STARK bytes that the rollup
    guest recursively verifies.
    """
    public_inputs: L2ExecutionProofPublicInput
    start_block_number: U64
    end_block_number: U64
    proof: bytes = b""
    l2_l1_messages: List[Hash32] = field(default_factory=list)
    tx_froms: List[Address] = field(default_factory=list)
    filtered_addresses: List[Address] = field(default_factory=list)


def run_l2_execution_guest(execution_input: L2ExecutionProofPrivateInput) -> L2ExecutionProof:
    """
    l2-execution: validates the EVM state transition for a contiguous block
    range and emits the 15-field l2-execution PI (§2.1).
    """
    if len(execution_input.blocks) == 0:
        raise Exception("l2-execution proof must cover at least one block")
    decoded_blocks = [
        decode_block_rlp(rollup_block.block_rlp)
        for rollup_block in execution_input.blocks
    ]
    if len(execution_input.execution_witnesses) != len(execution_input.blocks):
        raise Exception("execution witness count must match block count")

    parent_header = parent_header_from_execution_witness(
        decoded_blocks[0],
        execution_input.execution_witnesses[0],
    )
    parent_block_hash = block_hash(parent_header)
    start_block_number = parent_header.number + 1
    current_parent_header = parent_header
    l2_ms_address = execution_input.chain_config.l2_message_service_address

    # `base_fee` is sourced from the first block's header. The loop below
    # asserts every other block carries the same value, so any of them would
    # do; the first is canonical. (§2.1)
    base_fee = Uint(decoded_blocks[0].header.base_fee_per_gas)

    # Read the L1->L2 bridge rolling hash at the parent state root (start of
    # this range) via the EVM state interface. The witness pool must contain
    # the MPT path for the L2MessageService account + rolling-hash slots.
    parent_state = L2State(
        state_root=parent_header.state_root,
        witnesses=execution_input.execution_witnesses,
    )
    parent_rolling_hash, parent_rolling_hash_number = read_l1l2_bridge_state(
        parent_state, l2_ms_address,
    )

    current_ftx_rolling_hash = execution_input.parent_ftx_rolling_hash
    current_last_processed_ftx_number = execution_input.parent_last_processed_ftx_number
    l2_l1_message_hashes: List[Hash32] = []
    tx_froms: List[Address] = []
    filtered_addresses: List[Address] = []

    if decoded_blocks[0].header.number != start_block_number:
        raise Exception("l2-execution proof block range does not start after parent block")

    for rollup_block, execution_witness, block in zip(
        execution_input.blocks,
        execution_input.execution_witnesses,
        decoded_blocks,
    ):
        if Uint(block.header.base_fee_per_gas) != base_fee:
            raise Exception("baseFee must be constant across an l2-execution proof")
        if block.header.coinbase != execution_input.chain_config.coinbase:
            raise Exception("block coinbase does not match chain configuration")
        # Sequencer consensus rules: parent-hash chain & monotonic timestamps
        # (validate_header inside state_transition_modified covers the
        # standard Ethereum checks; here we restate the chain-level invariants).
        if block.header.parent_hash != block_hash(current_parent_header):
            raise Exception("block parent_hash does not chain from previous header")
        if Uint(block.header.timestamp) <= Uint(current_parent_header.timestamp):
            raise Exception("block timestamp must be strictly increasing")

        for tx_rlp in parse_block_transaction_rlps(rollup_block.block_rlp):
            tx_froms.append(
                # PRECOMPILE (production guest): secp256k1 ecrecover.
                # The zkVM exposes signature-recovery as a native circuit;
                # the Python reference defers to the execution-specs
                # implementation (which compiles to that primitive).
                recover_sender(
                    execution_input.chain_config.chain_id,
                    decode_signed_transaction_rlp(tx_rlp),
                ),
            )

        # FTX-invalid pre-validation reads the sender's account against the
        # L2 state at this block's parent state root (§6.5 'Invalid'). The
        # `L2State` interface is backed by the zkVM's MPT verifier
        # (PRECOMPILE: keccak256 for node hashing) over the witness pool.
        block_parent_state = L2State(
            state_root=current_parent_header.state_root,
            witnesses=execution_input.execution_witnesses,
        )
        block_filtered_addresses, current_ftx_rolling_hash, current_last_processed_ftx_number = (
            validate_forced_transactions(
                current_ftx_rolling_hash,
                current_last_processed_ftx_number,
                execution_input.chain_config,
                block.header,
                block_parent_state,
                rollup_block,
            )
        )
        filtered_addresses.extend(block_filtered_addresses)

        # PRECOMPILE-INTENSIVE (production guest): full EVM state transition.
        # `state_transition_modified` is the heart of the l2-execution proof —
        # it replays the block and verifies the resulting header (state root,
        # receipts root, transactions root, …). Internally it leans on
        # zkVM-native primitives for keccak256, secp256k1 ecrecover, sha256
        # (used by some precompiled contracts), modexp, BLS12-381 pairings
        # (point evaluation precompile on L1; here it's executed inside the
        # EVM), and MPT verification of every state/storage read.
        block_output = state_transition_modified(
            execution_input.chain_config.chain_id,
            current_parent_header,
            execution_witness,
            block,
            rollup_block.block_rlp,
        )
        current_parent_header = block.header
        current_block = block

        for log in block_output.block_logs:
            if log.address != execution_input.chain_config.l2_message_service_address:
                continue
            if log.topics[0] == BRIDGE_L2L1_MESSAGE_SENT_TOPIC_0:
                l2_l1_message_hashes.append(Hash32(log.topics[3]))

    # Read the L1->L2 bridge rolling hash at the end state root (after all
    # blocks in this range have applied) via the EVM state interface.
    end_state = L2State(
        state_root=current_block.header.state_root,
        witnesses=execution_input.execution_witnesses,
    )
    end_rolling_hash, end_rolling_hash_number = read_l1l2_bridge_state(
        end_state, l2_ms_address,
    )

    if end_rolling_hash_number < parent_rolling_hash_number:
        raise Exception("L1-to-L2 rolling-hash message number cannot decrease")

    public_inputs = L2ExecutionProofPublicInput(
        parent_block_hash=parent_block_hash,
        end_block_hash=block_hash(current_block.header),
        end_block_number=current_block.header.number,
        end_block_timestamp=U64(current_block.header.timestamp),
        l2_l1_messages_hash=hash_hash_list(l2_l1_message_hashes),
        parent_l1_l2_bridge_rolling_hash=parent_rolling_hash,
        parent_l1_l2_bridge_rolling_hash_message_number=parent_rolling_hash_number,
        end_l1_l2_bridge_rolling_hash=end_rolling_hash,
        end_l1_l2_bridge_rolling_hash_message_number=end_rolling_hash_number,
        dynamic_chain_config_hash=execution_input.chain_config.hash(base_fee),
        parent_ftx_rolling_hash=execution_input.parent_ftx_rolling_hash,
        end_ftx_rolling_hash=current_ftx_rolling_hash,
        last_processed_ftx_number=current_last_processed_ftx_number,
        filtered_addresses_hash=hash_address_list(filtered_addresses),
        tx_froms_hash=hash_address_list(tx_froms),
    )

    return L2ExecutionProof(
        public_inputs=public_inputs,
        start_block_number=start_block_number,
        end_block_number=current_block.header.number,
        l2_l1_messages=l2_l1_message_hashes,
        tx_froms=tx_froms,
        filtered_addresses=filtered_addresses,
    )


def parent_header_from_execution_witness(block: EthereumBlock, execution_witness: ExecutionWitness) -> Header:
    """
    Extract the parent header for the first execution block from the witness.

    The protocol witness supplies recent headers through
    `debug_executionWitness.headers`; the parent header is the one whose block
    hash equals the first block's `parent_hash`.
    """
    parent_headers = [
        header
        for header in execution_witness.headers
        if block_hash(header) == block.header.parent_hash
    ]
    if len(parent_headers) != 1:
        raise Exception("execution witness must contain exactly one parent header for the first block")
    return parent_headers[0]


def hash_hash_list(values: Sequence[Hash32]) -> Hash32:
    return keccak256(b"".join(bytes(value) for value in values))


def hash_address_list(values: Sequence[Address]) -> Hash32:
    return keccak256(b"".join(bytes(value) for value in values))


def uint256_topic_to_int(topic: Bytes32) -> int:
    return int.from_bytes(bytes(topic), "big")
