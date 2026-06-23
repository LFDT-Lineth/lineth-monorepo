from dataclasses import dataclass, field
from typing import List, Sequence, Tuple

from ethereum.crypto.hash import Hash32, keccak256
from .fork import (
    BlobTransaction,
    FeeMarketTransaction,
    SetCodeTransaction,
    recover_sender,
    calculate_total_blob_gas,
)
from ethereum.state import Address
from ethereum_types.bytes import Bytes32
from ethereum_types.numeric import U64, Uint

from .block import (
    ChainConfig,
    ExecutionPayload,
    ForcedTransactionAcceptance,
    ForcedTransactionWitness,
    LineaPayloadInput,
    ResolvedForcedTransaction,
    StatelessInput,
    decode_signed_transaction_rlp,
    parse_payload_transaction_rlps,
    resolve_forced_transaction,
)
from .stateless_input import decode_stateless_input_ssz
from .state_transition import (
    L2State,
    StatelessExecutionResult,
    execute_stateless_input,
)

BRIDGE_L2L1_MESSAGE_SENT_TOPIC_0 = Hash32(
    bytes.fromhex("e856c2b8bd4eb0027ce32eeaf595c21b0b6b4644b326e5b7bd80a1cf8db72e6c"),
)

# Storage layout of the L2MessageService contract.
#
# The L1->L2 rolling hash lives in the `l1RollingHashes` mapping keyed by message
# number, with the latest number in `lastAnchoredL1MessageNumber`. These slot
# indices are extracted from the compiled storage layout of
# `contracts/src/messaging/l2/L2MessageService.sol`. If that layout changes
# (including `__gap` slots in any ancestor) they MUST be re-extracted, or the
# proof reads wrong values and the L1 finalization check fails.
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
    Read the L1->L2 bridge rolling hash and its message number from the
    L2MessageService storage at `state.state_root` (via the EVM state interface;
    the guest verifies the MPT paths from `ExecutionWitness.state`).

    Two reads: `lastAnchoredL1MessageNumber` at a fixed slot, then
    `l1RollingHashes[thatNumber]` at `keccak256(uint256_be(number) || base_slot)`.
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
    payload: ExecutionPayload,
    parent_state: L2State,
    forced_transactions: Sequence[ForcedTransactionWitness],
) -> Tuple[List[Address], Hash32, U64]:
    """
    Scan the forced transactions declared for this payload and assert each has the
    correct outcome (Included / Invalid sub-case / Refused sub-case,
    §6.5). For the Invalid sub-cases, the FTX sender's account is read
    from `parent_state` (the L2 state at the parent of this block) via
    the EVM state interface; the witness pool in `ExecutionWitness.state`
    must include that MPT path.

    Dispatch on `resolved_ftx.acceptance`:
      - FILTERED_ADDRESS_FROM | FILTERED_ADDRESS_TO         -> Refused;
        bubble up the relevant address for the L1 sanction-list check.
      - INCLUDED                                            -> assert
        `txHash` is in `executionPayload.transactions`.
      - BAD_NONCE | BAD_BALANCE                              -> Invalid;
        assert `txHash` is NOT in the payload AND that the specific
        pre-validation failure holds against the sender's account read
        from `parent_state`.
    """
    rejected_addresses: List[Address] = []

    payload_tx_hashes = [
        keccak256(tx_rlp)
        for tx_rlp in parse_payload_transaction_rlps(payload)
    ]

    for ftx in forced_transactions:
        # FTXs are processed in ascending L1-assigned number.
        if ftx.number != last_processed_ftx_number + 1:
            raise Exception("forced transactions must be processed in ascending sequence")

        # Deadline constraint (§6.5): the FTX must be handled in a block whose
        # number does not exceed its declared deadline.
        if ftx.deadline < payload.block_number:
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

        # Payload-membership check: INCLUDED variants must appear in the
        # Engine API transaction list; the two Invalid variants must NOT.
        tx_in_block = resolved_ftx.tx_hash in payload_tx_hashes

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
    l2-execution guest input: one Linea wrapper per block in the conflation.

    Each wrapper's `stateless_input_ssz` is the raw vanilla stateless-input
    byte slice, decoded inside the guest path (no decoded-input fallback). The
    first input's witness must end with the parent header whose hash equals
    `executionPayload.parentHash`.

    The L1->L2 rolling-hash boundary values are not separate fields: the guest
    reads them from L2 state at the parent and end roots (`L2State.storage`), so
    the witness producer must include those MPT paths in `ExecutionWitness.state`.
    """
    parent_ftx_rolling_hash: Hash32
    parent_last_processed_ftx_number: U64
    payloads: List[LineaPayloadInput]
    chain_config: ChainConfig


def _decode_payload_stateless_inputs(payloads: Sequence[LineaPayloadInput]) -> List[StatelessInput]:
    """
    Decode the vanilla stateless-input SSZ bytes inside the guest path — matching
    the underlying engine's boundary, where the guest receives length-delimited
    byte slices, not pre-decoded objects.
    """
    decoded: List[StatelessInput] = []
    for index, payload in enumerate(payloads):
        try:
            decoded.append(decode_stateless_input_ssz(payload.stateless_input_ssz))
        except Exception as exc:
            raise Exception(f"invalid statelessInputSsz for payload {index}") from exc
    return decoded


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
    l2-execution: emits the 15-field l2-execution PI (§2.1) for a contiguous
    block range.

    The per-block state transition is delegated to the underlying engine
    (`execute_stateless_input`); this function adds only the Linea logic on top —
    conflation-level linking, the empty-`executionRequests` policy, forced
    transactions, L2->L1 messages, and the L1->L2 bridge rolling-hash reads.
    """
    if len(execution_input.payloads) == 0:
        raise Exception("l2-execution proof must cover at least one payload")

    # Parse each vanilla stateless input ONCE via the underlying engine's parser
    # (e.g. Zesu); the parsed objects are shared between execution and the Linea
    # logic below, so nothing is re-parsed.
    stateless_inputs = _decode_payload_stateless_inputs(execution_input.payloads)
    all_witnesses = [stateless_input.witness for stateless_input in stateless_inputs]

    first_payload = stateless_inputs[0].new_payload_request.execution_payload
    # The engine validates each payload's parentHash against its witness parent
    # header, so the range's parent block hash is the first payload's parentHash
    # and the start block number is the first payload's block number.
    parent_block_hash = first_payload.parent_hash
    start_block_number = first_payload.block_number
    base_fee = Uint(first_payload.base_fee_per_gas)  # asserted constant across the range (§2.1)
    l2_ms_address = execution_input.chain_config.l2_message_service_address

    current_parent_hash = parent_block_hash
    current_ftx_rolling_hash = execution_input.parent_ftx_rolling_hash
    current_last_processed_ftx_number = execution_input.parent_last_processed_ftx_number
    l2_l1_message_hashes: List[Hash32] = []
    tx_froms: List[Address] = []
    filtered_addresses: List[Address] = []
    results: List[StatelessExecutionResult] = []

    for linea_payload, stateless_input in zip(execution_input.payloads, stateless_inputs):
        payload = stateless_input.new_payload_request.execution_payload

        # ── Conflation-level invariants the engine cannot know (it validates each
        # block in isolation against its own witness parent) ──
        if stateless_input.chain_config.chain_id != execution_input.chain_config.chain_id:
            raise Exception("stateless input chain_id does not match proof-range chain configuration")
        if payload.parent_hash != current_parent_hash:
            raise Exception("payload parentHash does not chain from the previous payload")
        if Uint(payload.base_fee_per_gas) != base_fee:
            raise Exception("baseFee must be constant across an l2-execution proof")
        if payload.fee_recipient != execution_input.chain_config.coinbase:
            raise Exception("payload feeRecipient does not match chain configuration")
        # Monotonic timestamps and block-number contiguity follow from the
        # engine's per-block timestamp/parent checks plus the parentHash chaining
        # asserted above, so they are not restated here.

        # ── Linea policy: this rollup does not support EIP-7685 requests ──
        requests = stateless_input.new_payload_request.execution_requests
        if requests.deposits or requests.withdrawals or requests.consolidations:
            raise Exception("execution requests are not supported by this rollup")

        # ── State transition (delegated) ──
        # `execute_stateless_input` validates the witness header chain, the full
        # Engine-API payload, and replays the EVM (see its docstring); none of
        # that is re-checked here. It returns the boundary state roots and logs.
        result = execute_stateless_input(stateless_input)
        results.append(result)

        # Linea PI: recover each transaction sender for `txFromsHash`.
        for tx_rlp in parse_payload_transaction_rlps(payload):
            tx_froms.append(
                recover_sender(
                    execution_input.chain_config.chain_id,
                    decode_signed_transaction_rlp(tx_rlp),
                )
            )

        # Forced transactions (§6.5): FTX-invalid reads the sender account at this
        # block's parent state root by walking the witness MPT (`L2State`).
        block_parent_state = L2State(state_root=result.pre_state_root, witnesses=all_witnesses)
        block_filtered_addresses, current_ftx_rolling_hash, current_last_processed_ftx_number = (
            validate_forced_transactions(
                current_ftx_rolling_hash,
                current_last_processed_ftx_number,
                execution_input.chain_config,
                payload,
                block_parent_state,
                linea_payload.rollup_extension.forced_transactions,
            )
        )
        filtered_addresses.extend(block_filtered_addresses)

        # L2->L1 messages from the block's logs.
        for log in result.block_logs:
            if log.address != l2_ms_address:
                continue
            if log.topics[0] == BRIDGE_L2L1_MESSAGE_SENT_TOPIC_0:
                l2_l1_message_hashes.append(Hash32(log.topics[3]))

        current_parent_hash = payload.block_hash

    last_payload = stateless_inputs[-1].new_payload_request.execution_payload

    # L1->L2 bridge rolling-hash boundary reads, by walking the witness MPT
    # (`L2State`), at the range's parent (pre) and end (post) state roots.
    parent_rolling_hash, parent_rolling_hash_number = read_l1l2_bridge_state(
        L2State(state_root=results[0].pre_state_root, witnesses=all_witnesses), l2_ms_address,
    )
    end_rolling_hash, end_rolling_hash_number = read_l1l2_bridge_state(
        L2State(state_root=results[-1].post_state_root, witnesses=all_witnesses), l2_ms_address,
    )

    if end_rolling_hash_number < parent_rolling_hash_number:
        raise Exception("L1-to-L2 rolling-hash message number cannot decrease")

    public_inputs = L2ExecutionProofPublicInput(
        parent_block_hash=parent_block_hash,
        end_block_hash=last_payload.block_hash,
        end_block_number=last_payload.block_number,
        end_block_timestamp=U64(last_payload.timestamp),
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
        end_block_number=last_payload.block_number,
        l2_l1_messages=l2_l1_message_hashes,
        tx_froms=tx_froms,
        filtered_addresses=filtered_addresses,
    )


def hash_hash_list(values: Sequence[Hash32]) -> Hash32:
    return keccak256(b"".join(bytes(value) for value in values))


def hash_address_list(values: Sequence[Address]) -> Hash32:
    return keccak256(b"".join(bytes(value) for value in values))
