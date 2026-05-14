from ethereum.forks.osaka.blocks import Block as EthereumBlock, Header
from ethereum.forks.osaka.transactions import (
    Transaction, 
    encode_transaction,
    recover_sender, 
    validate_transaction,
    FeeMarketTransaction,
    BlobTransaction,
    SetCodeTransaction,
)
from ethereum.forks.osaka.fork import (
    BLOB_COUNT_LIMIT,
    VERSIONED_HASH_VERSION_KZG,
)
from ethereum.forks.osaka.vm.eoa_delegation import is_valid_delegation
from ethereum.forks.osaka.vm.gas import calculate_total_blob_gas
from ethereum.state import Address, Account, EMPTY_CODE_HASH
from dataclasses import dataclass
from ethereum_types.numeric import U64, Uint
from ethereum.crypto.hash import Hash32, keccak256
from ethereum_types.bytes import Bytes
from ethereum_rlp import rlp
from enum import Enum
from typing import List, Optional, Tuple


@dataclass
class ChainConfig:
    l2_message_service_address: Address
    base_fee: Uint
    coinbase: Address
    chain_id: U64

    def hash(self) -> Hash32:
        return keccak256(rlp.encode(self))

class ForcedTransactionAcceptance(Enum):
    """
    ForcedTransactionAcceptance is a enum indicating if a forced transaction has 
    been rejected. (rejection = arbitrary decision of the prover, but has to 
    report. The L1 contract will check if this is a reasonable decision). The 
    enum can be either

    - ACCEPTED
    - REJECTED_BECAUSE_FROM
    - REJECTED_BECAUSE_TO

    Note: if a transaction is NOT valid because of nonce/balance, it will be 
    tagged as accepted. The rejection is only for sanctionned addresses.
    """
    ACCEPTED=0
    REJECTED_BECAUSE_FROM=1
    REJECTED_BECAUSE_TO=2

@dataclass
class AccountWitness:
    """
    AccountWitness represents an account and its inclusion proof
    """
    account: Account
    address: Address
    # @alex: I haven't found a standard PMT proof for the account in the spec
    # repo but I might have just missed it. That's why the state-witness is just
    # place-holder list of hashes.
    state_witness: List[Hash32]
    code: Bytes
    """
    code is the code of the contract in case the sender account of the forced
    transaction is a contract.
    """

    def check_inclusion(self, state_root: Hash32) -> bool:
        """
        check_inclusion hashes the account, recovers the state_root hash and 
        equality-check the provided state_root with the recovered one.

        if the code is provided, it will also check that its hash matches the
        codehash in the account.
        """
        raise NotImplementedError("check_inclusion is not implemented")

@dataclass
class ForcedTransactionWitness:
    """
    ForcedTransactionWitness is the collection of input needed to validate the
    processing of a forced transaction. It consists of the content of the
    transaction, some indication on whether it is a sanctionned address and
    state witness from the from account to justify the invalidity of the 
    transaction.

    We expect the followings form:

    - acceptance == ACCEPTED && state_witness == NONE -> accepted and valid
    - acceptance != ACCEPTED && state_witness == NONE -> rejected
    - acceptance == ACCEPTED && state_witness != NONE -> invalid 
    """
    transaction: Transaction
    acceptance: ForcedTransactionAcceptance
    state_witness: Optional[AccountWitness]
    deadline: U64
    """
    deadline is an L2 block number. The forced transaction must be included in
    a block whose number is <= deadline; if not, it may be rejected and the
    sequencer is expected to report it.

    # @review: confirm that the deadline refers to L2 block numbers (not L1) and
    # that the comparison direction is correct (block.number > deadline means
    # the window has passed). Cross-check with the L1 contract enforcement logic.

    state_witness is provided to indicate and justify that the forced
    transaction was invalid. If the transaction is valid, then the value should
    be None.
    """

@dataclass
class RollupBlock:
    """
    RollupBlock wraps an Ethereum block and bundles it with forced-transactions
    informations which are supplementary.
    """
    ethereum_block: EthereumBlock
    forced_transactions: List[ForcedTransactionWitness]

def block_hash(header: Header) -> bytes:
    """
    block_hash computes the hash of a block header
    """
    return keccak256(rlp.encode(header))
    
def validate_forced_transactions(curr_rolling_hash: Hash32, 
                       curr_rolling_hash_message_number: U64,
                       chain_config: ChainConfig, state_root: Hash32, block: RollupBlock,
                       ) -> Tuple[List[Address], Hash32, U64]:
    """
    validate_forced_tx scans the forced transactions and checks they have been
    correctly executed/rejected.

    3 things can happen to a forced transaction:
        - It is REJECTED -> then we have to bubble-up the problematic address
        - It is included -> then we just check for inclusion in the block-list
        - Its Nonce/Balance make it an invalid transaction: 
    """

    rejected_addresses = []

    for ftx in block.forced_transactions:

        # Deadline check: a forced transaction whose deadline (L2 block number) is
        # strictly less than the current block number has expired. Note that this
        # spec iterates only over the FTXs present in the witness — it cannot
        # directly assert that *all* expired FTXs have been included. That liveness
        # guarantee is provided by the rolling hash: any omitted FTX would leave a
        # gap in the rolling hash chain, which the L1 contract detects when it
        # verifies newFtxRollingHash == ftxRollingHash[lastProcessedFtxNumber].
        if block.ethereum_block.header.number > ftx.deadline:
            raise Exception("deadline exceeded")
        
        #
        # The rolling hash is updated with the current forced transaction 
        # regardless of whether it was actually included or not.
        curr_rolling_hash, curr_rolling_hash_message_number = add_to_forced_tx_rolling_hash(
            curr_rolling_hash, curr_rolling_hash_message_number, ftx)

        from_address = recover_sender(chain_config.chain_id, ftx.transaction)
        if ftx.acceptance == ForcedTransactionAcceptance.REJECTED_BECAUSE_FROM:
            # The sequencer refuses the transaction on compliance grounds (sanctioned
            # sender). The from address is bubbled up; the L1 contract verifies a
            # posteriori that it appears in its reference sanction list. If it does
            # not, the finalization call is aborted — no explicit governance witness
            # is needed inside the proof.
            rejected_addresses.append(from_address)
            continue

        if ftx.acceptance == ForcedTransactionAcceptance.REJECTED_BECAUSE_TO:
            # Same as above, but the sanctioned party is the recipient. The to
            # address is bubbled up instead of the sender.
            #
            # Contract-creation transactions have no recipient (to == None).
            # REJECTED_BECAUSE_TO is meaningless for them and indicates a
            # malformed witness.
            if ftx.transaction.to is None:
                raise Exception("REJECTED_BECAUSE_TO on a contract-creation transaction")
            to_address: Address = ftx.transaction.to
            rejected_addresses.append(to_address)
            continue

        if ftx.state_witness is not None:

            if ftx.state_witness.address != from_address:
                raise Exception("state_witness provided for the wrong address") 

            if not ftx.state_witness.check_inclusion(state_root):
                raise Exception("provided an invalid state-inclusion witness for forced transactions")
            
            # This call will raise an exception if the transaction is 
            # intrisically invalid. But this does not cover situations where the
            # the transaction was invalid due to 
            #
            # @alex: normally these checks could be carried by the L1 contract
            # as they don't require state access. So we could change the design.
            # I still leave it here to make it explicit that we want to check
            # that one way or another.
            try:
                validate_transaction(ftx.transaction)
            except Exception:
                continue

            if is_valid_forced_transaction(ftx.transaction, ftx.state_witness, chain_config):
                raise Exception("the forced transaction was indicated to be invalid, but was found to be valid")
            continue

        # Otherwise, the transaction should be included in the block.
        # Per the spec, forced transactions must appear at the *beginning* of the
        # block's transaction list, before any regular sequencer transactions.
        # The check below only verifies set-inclusion; position enforcement is
        # left for a future, more precise implementation.
        #
        # @alex: this check should be better implemented and tested because I
        # don't think it works as is. I leave it still as it is easy to read and
        # good enough for a spec draft. As the reader, might have guessed we are
        # intending to check for inclusion of the forced transaction in the block.
        if ftx.acceptance != ForcedTransactionAcceptance.ACCEPTED: 
            raise Exception("forced transaction has an unknown acceptance value")
        if ftx.transaction not in block.ethereum_block.transactions:
            raise Exception("forced transaction was allegedly valid but not included")

    return rejected_addresses, curr_rolling_hash, curr_rolling_hash_message_number

def is_valid_forced_transaction(
        tx: Transaction, 
        sender_account_witness: AccountWitness,
        chain_config: ChainConfig,
    ) -> bool:
    """
    This function computes the cost of a forced transaction. It is essentially
    a copy-paste from [fork.check_transaction].

    It does essentially the same checks as [fork.check_transaction] except, all
    the checks relying on the remain gas/blob-gas in the block. Also, it returns
    a boolean indicating whether or not the transaction was valid. It does not
    raise errors.
    """
    sender_account = sender_account_witness.account
    if isinstance(
        tx, (FeeMarketTransaction, BlobTransaction, SetCodeTransaction)
    ):
        # @alex:
        # What if the forced transaction does not respect that? Shouldn't that
        # be part of an ad-hoc check? It could as well be done at L1 as we have
        # static baseFee.
        #
        if tx.max_fee_per_gas < tx.max_priority_fee_per_gas:
            return False

        if tx.max_fee_per_gas < chain_config.base_fee:
            return False
        max_gas_fee = tx.gas * tx.max_fee_per_gas
    else:
        if tx.gas_price < chain_config.base_fee:
            return False
        max_gas_fee = tx.gas * tx.gas_price

    if isinstance(tx, BlobTransaction):
        blob_count = len(tx.blob_versioned_hashes)
        if blob_count == 0:
            return False
        if blob_count > BLOB_COUNT_LIMIT:
            return False
        for blob_versioned_hash in tx.blob_versioned_hashes:
            if blob_versioned_hash[0:1] != VERSIONED_HASH_VERSION_KZG:
                return False

        max_gas_fee += Uint(calculate_total_blob_gas(tx)) * Uint(tx.max_fee_per_blob_gas)

    if isinstance(tx, (BlobTransaction, SetCodeTransaction)):
        if not isinstance(tx.to, Address):
            return False

    if isinstance(tx, SetCodeTransaction):
        if not any(tx.authorizations):
            return False

    if sender_account.nonce != Uint(tx.nonce):
        return False

    if Uint(sender_account.balance) < max_gas_fee + Uint(tx.value):
        return False
    sender_code = sender_account_witness.code
    if sender_account.code_hash != EMPTY_CODE_HASH and not is_valid_delegation(
        sender_code
    ):
        return False
    
    return True

def add_to_forced_tx_rolling_hash(forced_tx_rolling_hash: Hash32, 
                                  forced_tx_rolling_hash_message_number: U64,
                                  ftx: ForcedTransactionWitness) -> Tuple[Hash32, U64]:
    """
    add_to_forced_tx_rolling_hash updates the forced transaction rolling hash
    with ftx.

    Note: this formula differs from the one in the Readme, which uses
        keccak256(rollingHash ‖ keccak256(ftxRlp) ‖ deadline ‖ fromAddress)
    The Readme formula matches what the L1 contract computes. This simpler
    version (full tx bytes, no fromAddress) is kept here for readability and
    ease of implementation using the Ethereum spec primitives; it should be
    reconciled with the L1 contract formula before finalising the spec.
    """
    return keccak256(
        rlp.encode((
            forced_tx_rolling_hash,
            encode_transaction(ftx.transaction),
            ftx.deadline,
        ))
    ), forced_tx_rolling_hash_message_number + 1
