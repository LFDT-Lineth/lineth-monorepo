from ethereum.forks.osaka.fork import (
    BlockChain,
    MAX_RLP_BLOCK_SIZE,
    validate_header,
    get_last_256_block_hashes,
    apply_body,

)

from ethereum_rlp import rlp
from ethereum.exceptions import InvalidBlock
from ethereum.forks.osaka import vm
from ethereum.forks.osaka.blocks import Block
from ethereum.forks.osaka.bloom import logs_bloom
from ethereum.forks.osaka.requests import compute_requests_hash
from ethereum.forks.osaka.state import state_root
from ethereum.forks.osaka.trie import root

def state_transition_modified(chain: BlockChain, block: Block) -> vm.BlockOutput:
    """
    This function is exactly the same as the 
    [ethereum.forks.osaka.fork.state_transition] function but returns the 
    block_output as the above function does not do it. 
    """
    if len(rlp.encode(block)) > MAX_RLP_BLOCK_SIZE:
        raise InvalidBlock("Block rlp size exceeds MAX_RLP_BLOCK_SIZE")

    validate_header(chain, block.header)
    if block.ommers != ():
        raise InvalidBlock

    block_env = vm.BlockEnvironment(
        chain_id=chain.chain_id,
        state=chain.state,
        block_gas_limit=block.header.gas_limit,
        block_hashes=get_last_256_block_hashes(chain),
        coinbase=block.header.coinbase,
        number=block.header.number,
        base_fee_per_gas=block.header.base_fee_per_gas,
        time=block.header.timestamp,
        prev_randao=block.header.prev_randao,
        excess_blob_gas=block.header.excess_blob_gas,
        parent_beacon_block_root=block.header.parent_beacon_block_root,
    )

    block_output = apply_body(
        block_env=block_env,
        transactions=block.transactions,
        withdrawals=block.withdrawals,
    )
    block_state_root = state_root(block_env.state)
    transactions_root = root(block_output.transactions_trie)
    receipt_root = root(block_output.receipts_trie)
    block_logs_bloom = logs_bloom(block_output.block_logs)
    withdrawals_root = root(block_output.withdrawals_trie)
    requests_hash = compute_requests_hash(block_output.requests)

    if block_output.block_gas_used != block.header.gas_used:
        raise InvalidBlock(
            f"{block_output.block_gas_used} != {block.header.gas_used}"
        )
    if transactions_root != block.header.transactions_root:
        raise InvalidBlock
    if block_state_root != block.header.state_root:
        raise InvalidBlock
    if receipt_root != block.header.receipt_root:
        raise InvalidBlock
    if block_logs_bloom != block.header.bloom:
        raise InvalidBlock
    if withdrawals_root != block.header.withdrawals_root:
        raise InvalidBlock
    if block_output.blob_gas_used != block.header.blob_gas_used:
        raise InvalidBlock
    if requests_hash != block.header.requests_hash:
        raise InvalidBlock

    chain.blocks.append(block)
    if len(chain.blocks) > 255:
        # Real clients have to store more blocks to deal with reorgs, but the
        # protocol only requires the last 255
        chain.blocks = chain.blocks[-255:]

    return block_output