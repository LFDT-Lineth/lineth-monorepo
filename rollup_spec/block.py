from .fork import (
    Block as EthereumBlock,
    Header,
    Withdrawal,
    Transaction,
    recover_sender,
    LegacyTransaction,
    decode_transaction,
    ProtocolFork,
)
from ethereum.state import Address
from dataclasses import dataclass, field
from ethereum_types.numeric import U64, Uint
from ethereum.crypto.hash import Hash32, keccak256
from ethereum_types.bytes import Bytes, Bytes32
from ethereum_rlp import rlp
from enum import Enum
from typing import List, TypeAlias


BlockTransaction: TypeAlias = bytes | LegacyTransaction


@dataclass
class ChainConfig:
    """
    Static chain configuration that does not change across the proof range.

    `baseFee` is NOT part of this struct: the guest takes it from a block
    header (the first block in the range) and asserts every other block in
    the range carries the same `base_fee_per_gas`. The four-input
    `dynamicChainConfigHash` is then computed from `chainID`, `coinbase`,
    `l2MessageServiceAddress`, and that extracted `baseFee`.
    """
    l2_message_service_address: Address
    coinbase: Address
    chain_id: U64

    def hash(self, base_fee: Uint) -> Hash32:
        """
        Hash the dynamic chain configuration exposed by the l2-execution proof:

            keccak256(uint256_be(chainID) || coinbase || L2MessageServiceContract || uint256_be(baseFee))

        `base_fee` is supplied at hash time by the caller (it is sourced from
        the block header — see §2.1).
        """
        return keccak256(
            int(self.chain_id).to_bytes(32, "big") +
            bytes(self.coinbase) +
            bytes(self.l2_message_service_address) +
            int(base_fee).to_bytes(32, "big")
        )

class ForcedTransactionAcceptance(Enum):
    """
    Sequencer's declared outcome for a forced transaction.

    Mirrors the canonical Java enum at
    `linea-besu/plugins/linea-sequencer/sequencer/src/main/java/net/consensys/linea/sequencer/forced/ForcedTransactionInclusionResult.java`
    *narrowed to the cases that can actually be observed by the guest
    program under RISC-V proving*. Three variants from the Java enum are
    intentionally absent here:

      - `BadPrecompile` — every L2 precompile is just ordinary RISC-V code
        in the new design, so "disallowed precompile" can't fire.
      - `TooManyLogs`   — Linea's previous Type-2 stack imposed a per-tx
        log cap to keep the bespoke arithmetization tractable; the Type-1
        RISC-V stack has no such cap.
      - `Other`         — the canonical Java enum's transient-failure
        bucket, marked "should retry next block". It is never finalized,
        so the guest never sees it.

    Outcome category at proof level (§6.5):

    | Variant                | Category | Notes |
    |---|---|---|
    | INCLUDED               | Included | tx hash must appear in the host block's tx list |
    | BAD_NONCE              | Invalid  | sender's `account.nonce != tx.nonce` at the host block's parent state root |
    | BAD_BALANCE            | Invalid  | sender's `account.balance < tx.gasLimit * tx.maxFeePerGas + tx.value` |
    | FILTERED_ADDRESS_FROM  | Refused  | sender on the sanction list — bubble up sender address |
    | FILTERED_ADDRESS_TO    | Refused  | recipient on the sanction list — bubble up recipient address; rejected for contract-creation tx with `to == None` |
    """
    INCLUDED = 0
    BAD_NONCE = 1
    BAD_BALANCE = 2
    FILTERED_ADDRESS_FROM = 3
    FILTERED_ADDRESS_TO = 4

@dataclass
class ForcedTransactionWitness:
    """
    ForcedTransactionWitness is the collection of canonical input needed to
    validate the processing of a forced transaction. It consists of the signed
    transaction bytes, an L1-assigned number, the sequencer's acceptance
    decision (one of the canonical `ForcedTransactionAcceptance` variants
    above — Included / one of the Invalid sub-cases / one of the Refused
    sub-cases), and the L1-stored deadline.

    Dispatch in the proof (§6.5):

    - INCLUDED                       -> the guest asserts `txHash` is in the
                                         host block's transaction list.
    - BAD_NONCE / BAD_BALANCE        -> the guest asserts `txHash` is NOT in
                                         the block AND re-derives the
                                         specific pre-validation failure
                                         against the L2 state at the block's
                                         parent state root.
    - FILTERED_ADDRESS_FROM          -> the guest bubbles up `fromAddress`
                                         for L1-side sanction-list checking.
    - FILTERED_ADDRESS_TO            -> the guest bubbles up `toAddress`;
                                         rejected if the FTX is a
                                         contract-creation tx (`to == None`).
    """
    number: U64
    signed_tx_rlp: bytes
    acceptance: ForcedTransactionAcceptance
    deadline: U64
    """
    deadline is an L2 block number. The forced transaction must be handled
    (included, invalid, or refused) in a block whose number is <= deadline;
    if not, finalization past the deadline is blocked.
    """


@dataclass(frozen=True)
class ResolvedForcedTransaction:
    """
    Forced transaction after decoding its signed bytes and recovering sender.

    `ForcedTransactionWitness` carries only canonical inputs. This derived view
    is created once per witness and passed to downstream checks so sender
    recovery has a single owner.
    """
    number: U64
    signed_tx_rlp: bytes
    transaction: Transaction
    from_address: Address
    acceptance: ForcedTransactionAcceptance
    deadline: U64

    @property
    def tx_hash(self) -> Hash32:
        return keccak256(self.signed_tx_rlp)

@dataclass
class DepositRequest:
    """
    EIP-7685 deposit request carried by `NewPayloadRequest.executionRequests`.
    The SSZ decoder materializes this logical object from the guest input bytes.
    """
    pubkey: bytes
    withdrawal_credentials: Bytes32
    amount: U64
    signature: bytes
    index: U64


@dataclass
class WithdrawalRequest:
    """
    EIP-7002 withdrawal request carried by `NewPayloadRequest.executionRequests`.
    """
    source_address: Address
    validator_pubkey: bytes
    amount: U64


@dataclass
class ConsolidationRequest:
    """
    EIP-7251 consolidation request carried by `NewPayloadRequest.executionRequests`.
    """
    source_address: Address
    source_pubkey: bytes
    target_pubkey: bytes


@dataclass
class ExecutionRequests:
    """
    Typed Engine API execution requests.

    In SSZ these are bounded lists of fixed-size request containers, per
    EIP-8025. They are explicit here because the guest validates
    `NewPayloadRequest`, not a canonical block RLP.
    """
    deposits: List[DepositRequest] = field(default_factory=list)
    withdrawals: List[WithdrawalRequest] = field(default_factory=list)
    consolidations: List[ConsolidationRequest] = field(default_factory=list)


@dataclass
class StatelessChainConfig:
    """
    Chain context carried inside the stateless input.

    Linea still carries its proof-range `ChainConfig` outside the standard
    stateless payload because it also contains Linea-specific fields such as the
    L2MessageService address and coinbase. The guest must reject a payload whose
    stateless `chain_id` disagrees with the proof-range chain config. `active_fork`
    is the resolved `ProtocolFork` decoded from the SSZ `chain_config.active_fork`
    index (validated to be the spec's single supported fork — see `fork.py`).
    """
    chain_id: U64
    active_fork: ProtocolFork = ProtocolFork.Amsterdam


@dataclass
class ExecutionPayload:
    """
    Logical Engine API execution payload inside `NewPayloadRequest`.

    `transactions` are canonical signed tx bytes; the reference decodes an
    individual transaction only when it needs sender recovery or the
    execution-spec view.
    """
    parent_hash: Hash32
    fee_recipient: Address
    state_root: Hash32
    receipts_root: Hash32
    logs_bloom: bytes
    prev_randao: Bytes32
    block_number: U64
    gas_limit: Uint
    gas_used: Uint
    timestamp: U64
    extra_data: bytes
    base_fee_per_gas: Uint
    block_hash: Hash32
    transactions: List[bytes]
    withdrawals: List[Withdrawal]
    blob_gas_used: U64
    excess_blob_gas: U64
    block_access_list: bytes = b""
    slot_number: U64 | None = None


@dataclass
class NewPayloadRequest:
    """
    EIP-8025 payload request supplied to the l2-execution guest (replacing the
    old `blockRlp` input). Binds the proof to a concrete Engine API payload:
    versioned hashes, parent beacon block root, and typed execution requests.
    """
    execution_payload: ExecutionPayload
    versioned_hashes: List[Hash32]
    parent_beacon_block_root: Hash32
    execution_requests: ExecutionRequests = field(default_factory=ExecutionRequests)


@dataclass
class StatelessInput:
    """
    Decoded stateless input matching the underlying engine's guest boundary.

    Forced-transaction metadata is deliberately not here — it rides on
    `LineaPayloadInput.rollup_extension` so the stateless input stays vanilla.
    `public_keys` is decoded only because it is part of the stateless input; Linea
    derives signers via `recover_sender(chainID, tx)`, not from it.
    """
    new_payload_request: NewPayloadRequest
    witness: "ExecutionWitness"
    chain_config: StatelessChainConfig
    public_keys: List[bytes] = field(default_factory=list)


@dataclass
class LineaRollupExtension:
    """
    Linea-only fields beside the vanilla stateless input. Must not be appended to
    the stateless-input SSZ byte slice passed to the decoder.
    """
    forced_transactions: List[ForcedTransactionWitness] = field(default_factory=list)


@dataclass
class LineaPayloadInput:
    """
    One block of Linea l2-execution guest input.

    `stateless_input_ssz` is the vanilla stateless-input byte slice
    (length-delimited, decoded on its own); `rollup_extension` is Linea-only
    metadata consumed after payload execution, intentionally outside the
    stateless input.
    """
    stateless_input_ssz: bytes
    rollup_extension: LineaRollupExtension = field(default_factory=LineaRollupExtension)

def block_hash(header: Header) -> Hash32:
    """Hash of a block header."""
    return keccak256(rlp.encode(header))

def decode_block_rlp(block_rlp: bytes) -> EthereumBlock:
    """
    Decode a canonical block RLP carried by the rollup DA witness into the
    Ethereum execution-specs block view.
    """
    return rlp.decode_to(EthereumBlock, block_rlp)

def decode_signed_transaction_rlp(signed_tx_rlp: bytes) -> Transaction:
    """Decode a forced transaction's signed bytes (typed EIP-2718 or legacy)."""
    if len(signed_tx_rlp) == 0:
        raise Exception("empty signed transaction RLP")
    if signed_tx_rlp[0] in (1, 2, 3, 4):
        return decode_transaction(Bytes(signed_tx_rlp))
    return rlp.decode_to(LegacyTransaction, signed_tx_rlp)


def resolve_forced_transaction(ftx: ForcedTransactionWitness, chain_id: U64) -> ResolvedForcedTransaction:
    """
    Decode a forced transaction witness and recover its sender exactly once.
    """
    transaction = decode_signed_transaction_rlp(ftx.signed_tx_rlp)
    from_address = recover_sender(chain_id, transaction)
    return ResolvedForcedTransaction(
        number=ftx.number,
        signed_tx_rlp=ftx.signed_tx_rlp,
        transaction=transaction,
        from_address=from_address,
        acceptance=ftx.acceptance,
        deadline=ftx.deadline,
    )


def parse_payload_transaction_rlps(payload: ExecutionPayload) -> List[bytes]:
    """Return the signed transaction byte strings from an Engine API payload."""
    return [bytes(tx_rlp) for tx_rlp in payload.transactions]


def payload_transactions_for_execution(payload: ExecutionPayload) -> tuple[BlockTransaction, ...]:
    """
    Convert Engine API transaction bytes to the execution-spec `apply_body`
    shape: typed transactions stay as EIP-2718 bytes, legacy transactions are
    decoded to `LegacyTransaction`.
    """
    transactions: List[BlockTransaction] = []
    for tx_rlp in parse_payload_transaction_rlps(payload):
        if len(tx_rlp) == 0:
            raise Exception("empty transaction in payload")
        if tx_rlp[0] in (1, 2, 3, 4):
            transactions.append(tx_rlp)
        else:
            transactions.append(rlp.decode_to(LegacyTransaction, tx_rlp))
    return tuple(transactions)
