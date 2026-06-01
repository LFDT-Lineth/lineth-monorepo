from ethereum.forks.osaka.blocks import Block as EthereumBlock, Header
from ethereum.forks.osaka.transactions import (
    Transaction,
    recover_sender,
    LegacyTransaction,
    decode_transaction,
)
from ethereum.state import Address
from dataclasses import dataclass
from ethereum_types.numeric import U64, Uint
from ethereum.crypto.hash import Hash32, keccak256
from ethereum_types.bytes import Bytes
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
class RollupBlock:
    """
    Logical l2-execution block witness.

    `block_rlp` is the canonical private input supplied by the coordinator. The
    Python reference decodes an `EthereumBlock` from these bytes internally when
    it needs an execution view.
    """
    block_rlp: bytes
    forced_transactions: List[ForcedTransactionWitness]

def block_hash(header: Header) -> Hash32:
    """
    block_hash computes the hash of a block header
    """
    return keccak256(rlp.encode(header))

def decode_block_rlp(block_rlp: bytes) -> EthereumBlock:
    """
    Decode the canonical block RLP carried by the l2-execution private input
    into the Ethereum execution-specs block view.
    """
    return rlp.decode_to(EthereumBlock, block_rlp)

def decode_signed_transaction_rlp(signed_tx_rlp: bytes) -> Transaction:
    """
    Decode the canonical signed transaction bytes used for forced transactions.

    Typed EIP-2718 transactions are encoded as type byte || RLP(payload). Legacy
    transactions are plain RLP.
    """
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


def _canonical_transaction_bytes(transaction: BlockTransaction) -> bytes:
    """
    Return the canonical signed bytes for a transaction from an execution-spec
    block view.

    `Block.transactions` is `Tuple[bytes | LegacyTransaction, ...]`: typed
    transactions are already EIP-2718 bytes, while legacy transactions are
    decoded objects and need regular RLP encoding.
    """
    match transaction:
        case bytes():
            return transaction
        case LegacyTransaction():
            return rlp.encode(transaction)


def parse_block_transaction_rlps(block_rlp: bytes) -> List[bytes]:
    """
    Return the signed transaction RLP byte strings decoded from `block_rlp`.

    `blockRlp` is the canonical witness bytes in the logical schema. Decoded
    `EthereumBlock` objects are only execution views, so hash checks over block
    transactions must use bytes extracted from `block_rlp` rather than
    re-encoding decoded transaction objects.
    """
    block = decode_block_rlp(block_rlp)
    return [
        _canonical_transaction_bytes(transaction)
        for transaction in block.transactions
    ]
