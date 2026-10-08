from dataclasses import dataclass, field
from typing import List, Sequence, Tuple

from ethereum.crypto.hash import Hash32, keccak256
from .fork import (
    BlobTransaction,
    FeeMarketTransaction,
    Log,
    SetCodeTransaction,
    recover_sender,
    calculate_total_blob_gas,
)
from ethereum.state import Address
from ethereum_types.numeric import U64, Uint

from .block import (
    ChainConfig,
    ExecutionPayload,
    ForcedTransactionAcceptance,
    ForcedTransactionWitness,
    LinethPayloadInput,
    ResolvedForcedTransaction,
    StatelessInput,
    decode_signed_transaction_rlp,
    parse_payload_transaction_rlps,
    resolve_forced_transaction,
)
from .stateless_input import decode_stateless_input_ssz
from .state_transition import (
    L2State,
    execute_stateless_input,
)

BRIDGE_L2L1_MESSAGE_SENT_TOPIC_0 = Hash32(
    bytes.fromhex("e856c2b8bd4eb0027ce32eeaf595c21b0b6b4644b326e5b7bd80a1cf8db72e6c"),
)
BRIDGE_L1L2_ROLLING_HASH_UPDATED_TOPIC_0 = Hash32(
    bytes.fromhex("99b65a4301b38c09fb6a5f27052d73e8372bbe8f6779d678bfe8a41b66cce7ac"),
)

# Sentinel meaning "no L2MessageService configured" (see `_is_zero_address`).
ZERO_ADDRESS: Address = Address(b"\x00" * 20)
ZERO_HASH: Hash32 = Hash32(b"\x00" * 32)


def _is_zero_address(address: Address) -> bool:
    """True for the all-zero 20-byte address — the "no L2MessageService" sentinel."""
    return bytes(address) == bytes(ZERO_ADDRESS)


@dataclass
class BridgeLogScan:
    l2_l1_messages: List[Hash32]
    l2_messaging_blocks_offsets: List[int]
    end_l1_l2_bridge_rolling_hash: Hash32
    end_l1_l2_bridge_rolling_hash_message_number: U64


def scan_bridge_logs(
    l2_message_service_address: Address,
    parent_l1_l2_bridge_rolling_hash: Hash32,
    parent_l1_l2_bridge_rolling_hash_message_number: U64,
    block_logs: Sequence[Sequence[Log]],
) -> BridgeLogScan:
    """
    Collect L2->L1 messages and fold L1->L2 rolling-hash updates from the L2MessageService
    logs of a block range; `block_logs[i]` holds block i's ordered logs.

    `MessageSent` hashes are returned in log order, with the 1-based offset of every block
    that emitted at least one. `RollingHashUpdated` events advance the bridge pair seeded
    from the parent pair; their message numbers must strictly increase.

    "No L2MessageService configured" mode: a zero address skips the scan entirely, so
    nothing is collected and the parent pair passes through unchanged as the end pair.
    """
    if _is_zero_address(l2_message_service_address):
        return BridgeLogScan(
            [],
            [],
            parent_l1_l2_bridge_rolling_hash,
            parent_l1_l2_bridge_rolling_hash_message_number,
        )

    l2_l1_messages: List[Hash32] = []
    messaging_offsets: List[int] = []
    bridge_rolling_hash = parent_l1_l2_bridge_rolling_hash
    bridge_message_number = parent_l1_l2_bridge_rolling_hash_message_number

    for block_index, logs in enumerate(block_logs):
        has_message = False
        for log in logs:
            if log.address != l2_message_service_address:
                continue
            if log.topics and log.topics[0] == BRIDGE_L2L1_MESSAGE_SENT_TOPIC_0:
                if len(log.topics) < 4:
                    raise Exception("MessageSent log is missing its message hash topic")
                l2_l1_messages.append(Hash32(log.topics[3]))
                has_message = True
            elif log.topics and log.topics[0] == BRIDGE_L1L2_ROLLING_HASH_UPDATED_TOPIC_0:
                if len(log.topics) < 3:
                    raise Exception("RollingHashUpdated log is missing its topics")
                topic_number = int.from_bytes(bytes(log.topics[1]), "big")
                if topic_number > 0xFFFFFFFFFFFFFFFF:
                    raise Exception("L1-to-L2 rolling-hash message number exceeds uint64")
                message_number = U64(topic_number)
                if message_number <= bridge_message_number:
                    raise Exception("L1-to-L2 rolling-hash message number must increase")
                bridge_message_number = message_number
                bridge_rolling_hash = Hash32(log.topics[2])
        if has_message:
            if block_index + 1 > 0xFFFF:
                raise Exception("messaging block offset exceeds uint16")
            messaging_offsets.append(block_index + 1)

    return BridgeLogScan(l2_l1_messages, messaging_offsets, bridge_rolling_hash, bridge_message_number)


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
    The l2-execution public input tuple from Readme.md section 2.1.
    """
    parent_block_hash: Hash32
    end_block_hash: Hash32
    end_block_number: U64
    end_block_timestamp: U64
    l2_l1_messages: List[Hash32]
    parent_l1_l2_bridge_rolling_hash: Hash32
    parent_l1_l2_bridge_rolling_hash_message_number: U64
    end_l1_l2_bridge_rolling_hash: Hash32
    end_l1_l2_bridge_rolling_hash_message_number: U64
    dynamic_chain_config_hash: Hash32
    parent_ftx_rolling_hash: Hash32
    parent_ftx_number: U64
    end_ftx_rolling_hash: Hash32
    end_processed_ftx_number: U64
    filtered_addresses_hash: Hash32
    tx_froms_hash: Hash32
    block_count: int = 0
    l2_messaging_blocks_offsets: List[int] = field(default_factory=list)


@dataclass
class L2ExecutionProofPrivateInput:
    """
    l2-execution guest input: one Lineth wrapper per block in the conflation.

    Each wrapper's `stateless_input_ssz` is the raw vanilla stateless-input
    byte slice, decoded inside the guest path (no decoded-input fallback). The
    first input's witness must end with the parent header whose hash equals
    `executionPayload.parentHash`.

    The parent forced-transaction and L1->L2 bridge rolling-hash values are
    inputs, anchored by rollup/aggregation continuity and the L1 finalized-pair
    check.
    """
    parent_ftx_rolling_hash: Hash32
    parent_last_processed_ftx_number: U64
    parent_l1_l2_bridge_rolling_hash: Hash32
    parent_l1_l2_bridge_rolling_hash_message_number: U64
    payloads: List[LinethPayloadInput]
    chain_config: ChainConfig


def _decode_payload_stateless_inputs(payloads: Sequence[LinethPayloadInput]) -> List[StatelessInput]:
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
            raise Exception(f"invalid stateless input SSZ for payload {index}") from exc
    return decoded


@dataclass
class L2ExecutionProof:
    """
    An l2-execution proof with guest public inputs and host range metadata.

    Guest/prover boundary: the guest commits to `public_inputs`; `proof` is attached by the zkVM/prover layer above — a guest
    cannot prove itself — and is a placeholder (`b""`) in this reference.

    `end_block_number` is intentionally absent: it is already
    `public_inputs.end_block_number`. Only `start_block_number` (not in the PI
    tuple) is carried, so the rollup guest can verify proof tiling.

    This type has no verifying-key field: a guest cannot attest its own VK, so
    `run_l2_execution_guest` never produces one. See `VerifiableL2ExecutionProof`
    for the coordinator-populated wrapper the rollup guest actually consumes.
    """
    public_inputs: L2ExecutionProofPublicInput
    start_block_number: U64
    proof: bytes = b""
    filtered_addresses: List[Address] = field(default_factory=list)


@dataclass
class VerifiableL2ExecutionProof:
    """
    An `L2ExecutionProof` paired with the `program_vk` the rollup guest
    recursively verifies it against (§ProgramVK anchoring).

    `program_vk` is a *runtime input* the coordinator supplies — the same
    value it verifies `proof` against here is the value bubbled up into the
    rollup guest's `program_vks` public output, so the anchored VK is provably
    the key the verification ran against. Never produced by
    `run_l2_execution_guest` (a guest cannot attest its own VK); only the
    rollup request codec constructs this wrapper.
    """
    proof: L2ExecutionProof
    program_vk: Hash32


def run_l2_execution_guest(execution_input: L2ExecutionProofPrivateInput) -> L2ExecutionProof:
    """
    l2-execution: emits the l2-execution PI (§2.1) for a contiguous
    block range.

    The per-block state transition is delegated to the underlying engine
    (`execute_stateless_input`); this function adds only the Lineth logic on top —
    conflation-level linking, the empty-`executionRequests` policy, forced
    transactions, L2->L1 messages, and the L1->L2 bridge rolling-hash updates.
    """
    if len(execution_input.payloads) == 0:
        raise Exception("l2-execution proof must cover at least one payload")

    # Parse each vanilla stateless input ONCE via the underlying engine's parser
    # (e.g. Zesu); the parsed objects are shared between execution and the Lineth
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

    current_parent_hash = parent_block_hash
    current_ftx_rolling_hash = execution_input.parent_ftx_rolling_hash
    current_last_processed_ftx_number = execution_input.parent_last_processed_ftx_number
    block_logs: List[Sequence[Log]] = []
    tx_froms: List[Address] = []
    filtered_addresses: List[Address] = []

    for block_index, (lineth_payload, stateless_input) in enumerate(zip(execution_input.payloads, stateless_inputs)):
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

        # ── Lineth policy: this rollup does not support EIP-7685 requests ──
        requests = stateless_input.new_payload_request.execution_requests
        if requests.deposits or requests.withdrawals or requests.consolidations:
            raise Exception("execution requests are not supported by this rollup")

        # ── Linea policy: no beacon-chain withdrawals — this is an L2 rollup, not L1 ──
        if payload.withdrawals:
            raise Exception("withdrawals are not supported by this rollup")

        # ── State transition (delegated) ──
        # `execute_stateless_input` validates the witness header chain, the full
        # Engine-API payload, and replays the EVM (see its docstring); none of
        # that is re-checked here. It returns the boundary state roots and logs.
        result = execute_stateless_input(stateless_input)

        # Lineth PI: recover each transaction sender for `txFromsHash`.
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
                lineth_payload.rollup_extension.forced_transactions,
            )
        )
        filtered_addresses.extend(block_filtered_addresses)

        block_logs.append(result.block_logs)

        current_parent_hash = payload.block_hash

    last_payload = stateless_inputs[-1].new_payload_request.execution_payload

    bridge_scan = scan_bridge_logs(
        execution_input.chain_config.l2_message_service_address,
        execution_input.parent_l1_l2_bridge_rolling_hash,
        execution_input.parent_l1_l2_bridge_rolling_hash_message_number,
        block_logs,
    )

    public_inputs = L2ExecutionProofPublicInput(
        parent_block_hash=parent_block_hash,
        end_block_hash=last_payload.block_hash,
        end_block_number=last_payload.block_number,
        end_block_timestamp=U64(last_payload.timestamp),
        l2_l1_messages=bridge_scan.l2_l1_messages,
        parent_l1_l2_bridge_rolling_hash=execution_input.parent_l1_l2_bridge_rolling_hash,
        parent_l1_l2_bridge_rolling_hash_message_number=execution_input.parent_l1_l2_bridge_rolling_hash_message_number,
        end_l1_l2_bridge_rolling_hash=bridge_scan.end_l1_l2_bridge_rolling_hash,
        end_l1_l2_bridge_rolling_hash_message_number=bridge_scan.end_l1_l2_bridge_rolling_hash_message_number,
        dynamic_chain_config_hash=execution_input.chain_config.hash(base_fee),
        parent_ftx_rolling_hash=execution_input.parent_ftx_rolling_hash,
        parent_ftx_number=execution_input.parent_last_processed_ftx_number,
        end_ftx_rolling_hash=current_ftx_rolling_hash,
        end_processed_ftx_number=current_last_processed_ftx_number,
        filtered_addresses_hash=hash_address_list(filtered_addresses),
        tx_froms_hash=hash_address_list(tx_froms),
        block_count=len(stateless_inputs),
        l2_messaging_blocks_offsets=bridge_scan.l2_messaging_blocks_offsets,
    )

    return L2ExecutionProof(
        public_inputs=public_inputs,
        start_block_number=start_block_number,
        filtered_addresses=filtered_addresses,
    )


def hash_digest_list(values: Sequence[Hash32]) -> Hash32:
    return keccak256(b"".join(bytes(value) for value in values))


def hash_address_list(values: Sequence[Address]) -> Hash32:
    return keccak256(b"".join(bytes(value) for value in values))
