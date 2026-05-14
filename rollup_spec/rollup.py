from ethereum.forks.osaka.blocks import Block as EthereumBlock
from ethereum.crypto.hash import keccak256, Hash32
from ethereum_rlp import rlp
from ethereum_types.numeric import U64
from ethereum_types.bytes import Bytes32
from ethereum.state import Address
from ethereum.forks.osaka.fork import BlockChain
from .blob import ShnarfWitness, RollupDataWitness
from .block import RollupBlock, block_hash, validate_forced_transactions, ChainConfig
from typing import List
from dataclasses import dataclass
from .state_transition import state_transition_modified

BRIDGE_L2L1_MESSAGE_SENT_TOPIC_0 = Bytes32(bytes.fromhex("e856c2b8bd4eb0027ce32eeaf595c21b0b6b4644b326e5b7bd80a1cf8db72e6c"))
BRIDGE_L1L2_ROLLING_HASH_UPDATED_TOPIC_0 = Bytes32(bytes.fromhex("99b65a4301b38c09fb6a5f27052d73e8372bbe8f6779d678bfe8a41b66cce7ac"))

@dataclass
class RollupPrivateInput:
    """
    RollupInput specifies the input of the rollup execution
    """
    prev_finalized_shnarf_witness: ShnarfWitness
    prev_finalized_blockchain: BlockChain
    prev_finalized_bridge_l1l2_rolling_hash: Hash32
    prev_finalized_bridge_l1l2_rolling_hash_message_number: U64
    prev_finalized_forced_tx_rolling_hash: Hash32
    prev_finalized_forced_tx_rolling_hash_message_number: U64
    blocks: list[RollupBlock]
    das: list[RollupDataWitness]
    chain_config: ChainConfig

# RollupPrivateInput specified the input of the rollup execution
@dataclass
class RollupPublicInput:
    #
    # Dynamic chain configuration: H(coinBase, l2Bridge, chainID, baseFee)
    dynamic_chain_config_hash: bytes 
    #
    # Marks the transition
    prev_finalized_shnarf: bytes 
    final_shnarf: bytes 
    #
    # These are just for the L1 contract to access easily.
    final_block_number: int 
    final_state_root: bytes 
    #
    # L1 -> L2 message anchoring feedback loop
    prev_bridge_l1l2_rolling_hash: bytes
    prev_bridge_l1l2_rolling_hash_message_number: int
    bridge_l1l2_rolling_hash: bytes 
    bridge_l1l2_rolling_hash_message_number: int 
    #
    # L2 -> L1 message delivery
    bridge_l2l1_transaction_tree: list[Bytes32] 
    #
    # Forced-transactions info
    prev_forced_tx_rolling_hash: bytes 
    next_forced_tx_rolling_hash: bytes 
    last_processed_forced_tx_number: int 
    filtered_addresses_hash: bytes 


def check_rollup_validity(rollup_input: RollupPrivateInput) -> RollupPublicInput:
    """
    check_rollup_validity is the entrypoint of the rollup validation function.
    """

    curr_block: EthereumBlock
    curr_block, _, is_valid_blockchain = validate_block_history(rollup_input.prev_finalized_blockchain)
    if not is_valid_blockchain:
        raise Exception("invalid block history: the block hashes are not in sequences")

    curr_shnarf: Hash32 = rollup_input.prev_finalized_shnarf_witness.hash()
    if block_hash(curr_block.header) != rollup_input.prev_finalized_shnarf_witness.block_hash:
        raise Exception("Prev finalized block and prev finalized shnarf witness are not consistent")
    prev_finalized_shnarf = curr_shnarf

    forced_tx_rejected_addresses: list[Address] = []
    bridge_l2l1_message_hashes: list[Bytes32] = []
    curr_bridge_l1l2_rolling_hash = rollup_input.prev_finalized_bridge_l1l2_rolling_hash
    curr_bridge_l1l2_rolling_hash_message_number = rollup_input.prev_finalized_bridge_l1l2_rolling_hash_message_number
    curr_forced_tx_rolling_hash = rollup_input.prev_finalized_forced_tx_rolling_hash
    curr_forced_tx_rolling_hash_message_number = rollup_input.prev_finalized_forced_tx_rolling_hash_message_number
    # initial block number is the first block number of the sequence of block to
    # be validated.
    initial_block_number = curr_block.header.number + 1

    total_blocks = sum(
        da.block_number_range[1] + 1 - da.block_number_range[0]
        for da in rollup_input.das
    )
    if len(rollup_input.blocks) != total_blocks:
        raise Exception(
            f"rollup_input.blocks has {len(rollup_input.blocks)} entries but "
            f"the DA blobs span {total_blocks} blocks"
        )

    for blob in rollup_input.das:

        blob_auth, blob_hash = blob.is_authenticated_blob_bytes()
        if not blob_auth:
            raise Exception("Invalid KZG proof")
       
        blob_blocks_data = blob.parse_block_data()
        if blob.block_number_range[1] + 1 - blob.block_number_range[0] != len(blob_blocks_data):
            raise Exception("block range is inconsistent with decompressed block data")
        # Note: continuity between consecutive blobs (i.e. blob[i+1].block_number_range[0]
        # == blob[i].block_number_range[1] + 1) is not checked explicitly here. It is
        # enforced implicitly by state_transition_modified via validate_header, which
        # requires header.number == parent.number + 1 and header.parent_hash ==
        # keccak256(rlp(parent_header)). Any gap or overlap would cause the first block
        # of the offending blob to fail header validation.

        for i in range(len(blob_blocks_data)):
            exec_block_position = blob.block_number_range[0] - initial_block_number + i
            exec_block_data: RollupBlock = rollup_input.blocks[exec_block_position]
            blob_block_data = blob_blocks_data[i]
            #
            # This step implicitly checks the signatures when comparing the from
            # from the addresses of the truncated block.
            if not blob_block_data.is_consistent_with(exec_block_data.ethereum_block, rollup_input.chain_config.chain_id):
                raise Exception("Invalid block consistency")
            rejected_addresses, curr_forced_tx_rolling_hash, curr_forced_tx_rolling_hash_message_number = validate_forced_transactions(
                    curr_forced_tx_rolling_hash,
                    curr_forced_tx_rolling_hash_message_number,
                    rollup_input.chain_config,
                    curr_block.header.state_root,
                    exec_block_data,
                )
            forced_tx_rejected_addresses.extend(rejected_addresses)
            #
            # This runs and checks the state-transition function of Ethereum.
            # In case the transition function is invalid the function will raise
            # an error.
            #
            # This function also "bump" the blockchain object. So the forced
            # transaction checks have to be done before to be sure we use the
            # prior state as a reference.
            block_output = state_transition_modified(rollup_input.prev_finalized_blockchain, exec_block_data.ethereum_block)
            curr_block = exec_block_data.ethereum_block
            #
            # This steps accumulates the L2->L1 bridge messages and the L1->L2
            # bridge messages.
            # @alex: this is all pseudo-code for now
            for log in block_output.block_logs:
                if log.address != rollup_input.chain_config.l2_message_service_address:
                    continue
                if log.topics[0] == BRIDGE_L2L1_MESSAGE_SENT_TOPIC_0:
                    #
                    # The event signature is
                    #
                    #   event MessageSent(
                    #       address indexed _from,
                    #       address indexed _to,
                    #       uint256 _fee,
                    #       uint256 _value,
                    #       uint256 _nonce,
                    #       bytes _calldata,
                    #       bytes32 indexed _messageHash
                    #   );
                    #
                    # and we want to extract the message hash.
                    bridge_l2l1_message_hashes.append(log.topics[3])
                if log.topics[0] == BRIDGE_L1L2_ROLLING_HASH_UPDATED_TOPIC_0:
                    #
                    # The event signature is
                    #
                    #   RollingHashUpdated(
                    #       uint256 indexed messageNumber, 
                    #       bytes32 indexed rollingHash,
                    #   );
                    #
                    # and we want to extract the rolling hash and the message number.
                    new_message_number = log.topics[1]
                    new_rolling_hash = log.topics[2]
                    if new_message_number < curr_bridge_l1l2_rolling_hash_message_number + 1:
                        raise Exception("incompatible previous rolling hash number")
                    curr_bridge_l1l2_rolling_hash = new_rolling_hash
                    curr_bridge_l1l2_rolling_hash_message_number = new_message_number
        #
        # To wrap up the per-blob checking, we compute the new value of the
        # shnarf. Note: the shnarf commits only to the *last* block of each blob
        # (via its block_hash) and to the blob as a whole (via blob_hash).
        # Intermediate blocks within a blob are committed exclusively through the
        # DA blob itself — they are not individually chained into the shnarf. This
        # is intentional: the blob_hash already authenticates the full block sequence
        # inside the blob via the KZG proof above.
        curr_shnarf = ShnarfWitness(
            curr_shnarf,
            block_hash(exec_block_data.ethereum_block.header),
            blob_hash,
        ).hash()

    return RollupPublicInput(dynamic_chain_config_hash=rollup_input.chain_config.hash(),
                             prev_finalized_shnarf=prev_finalized_shnarf,
                             final_shnarf=curr_shnarf,
                             final_block_number=curr_block.header.number,
                             final_state_root=curr_block.header.state_root,
                             prev_bridge_l1l2_rolling_hash=rollup_input.prev_finalized_bridge_l1l2_rolling_hash,
                             prev_bridge_l1l2_rolling_hash_message_number=rollup_input.prev_finalized_bridge_l1l2_rolling_hash_message_number,
                             bridge_l1l2_rolling_hash=curr_bridge_l1l2_rolling_hash,
                             bridge_l1l2_rolling_hash_message_number=curr_bridge_l1l2_rolling_hash_message_number,
                             bridge_l2l1_transaction_tree=build_l2_messages_tree(bridge_l2l1_message_hashes),
                             prev_forced_tx_rolling_hash=rollup_input.prev_finalized_forced_tx_rolling_hash,
                             next_forced_tx_rolling_hash=curr_forced_tx_rolling_hash,
                             last_processed_forced_tx_number=curr_forced_tx_rolling_hash_message_number,
                             filtered_addresses_hash=keccak256(rlp.encode(forced_tx_rejected_addresses)),
    )

def validate_block_history(blockchain: BlockChain) -> tuple[EthereumBlock, Hash32, bool]:
    """
    validate_block_history checks that the provided blockchain has blocks in 
    sequence. It does not check the consensus rules of it as they have normally
    already been checked by a previous rollup proof.

    The function returns the last block of the chain and its hash alongside with
    a boolean 
    """
    blocks = blockchain.blocks
    assert len(blocks) > 0, "blockchain must contain at least the genesis block"
    parent_block_hash = blocks[0].header.parent_hash
    for block in blocks:
        if block.header.parent_hash != parent_block_hash:
            return None, None, False
        parent_block_hash = block_hash(block.header)
    return blocks[-1], parent_block_hash, True


def build_l2_messages_tree(msgs: List[Hash32]) -> List[Hash32]:
    """
    build_l2_messages_tree works as follows:
    - Pads the list of msgs with Bytes32 until it reaches a multiple of 32
    - Merkle hashes each segment of 32 message in a complete merkle tree of
        depth 5 (2^5 = 32 leaves)
    - Flat hash the roots into a single hash.

    The hashing is done using the keccak hash function.
    """
    raise NotImplementedError("just a placeholder implementation")




