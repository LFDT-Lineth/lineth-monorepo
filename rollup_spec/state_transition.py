from dataclasses import dataclass
from functools import cached_property
from typing import Dict, List, Optional, Sequence, Tuple

from ethereum.forks.osaka.fork import (
    BlockChain,
    MAX_RLP_BLOCK_SIZE,
    validate_header,
    get_last_256_block_hashes,
    apply_body,
)

from ethereum.exceptions import InvalidBlock
from ethereum.forks.osaka import vm
from ethereum.forks.osaka.blocks import Block, Header
from ethereum.forks.osaka.bloom import logs_bloom
from ethereum.forks.osaka.requests import compute_requests_hash
from ethereum.state import Address, Account, state_root
from ethereum.crypto.hash import Hash32, keccak256
from ethereum.merkle_patricia_trie import root
from ethereum_rlp import rlp
from ethereum_types.bytes import Bytes, Bytes32
from ethereum_types.numeric import U64, U256, Uint

@dataclass
class ExecutionWitness:
    """
    Logical form of Besu's `debug_executionWitness` payload for one block.

    The binary/JSON schema carries these fields as encoded bytes. The Python
    reference treats headers as decoded `Header` objects so it can state the
    parent-header matching rule directly.

    The `state` pool must include MPT paths for every account/slot the
    l2-execution guest reads — both what block execution naturally touches
    and any extra reads the guest performs (e.g., L1->L2 rolling-hash slots at
    boundary state roots, FTX-sender accounts for §6.5 'Invalid' checks).
    """
    state: List[bytes]
    codes: List[bytes]
    keys: List[bytes]
    headers: List[Header]


# keccak256(rlp.encode(b"")) — the canonical Ethereum "empty trie" root.
EMPTY_TRIE_ROOT_HASH: Hash32 = Hash32(
    bytes.fromhex("56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"),
)


def _build_node_index(witnesses: Sequence["ExecutionWitness"]) -> Dict[Hash32, bytes]:
    """Build `{keccak256(node_rlp) -> node_rlp}` over `witnesses[*].state`."""
    index: Dict[Hash32, bytes] = {}
    for w in witnesses:
        for node in w.state:
            index[Hash32(keccak256(node))] = bytes(node)
    return index


def _build_code_index(witnesses: Sequence["ExecutionWitness"]) -> Dict[Hash32, bytes]:
    """Build `{keccak256(code) -> code}` over `witnesses[*].codes`."""
    index: Dict[Hash32, bytes] = {}
    for w in witnesses:
        for code in w.codes:
            index[Hash32(keccak256(code))] = bytes(code)
    return index


def _bytes_to_nibbles(data: bytes) -> List[int]:
    """Expand a byte string into a flat list of 4-bit nibbles, high first."""
    out: List[int] = []
    for b in data:
        out.append((b >> 4) & 0x0F)
        out.append(b & 0x0F)
    return out


def _compact_to_nibbles(compact: bytes) -> Tuple[List[int], bool]:
    """
    Decode the Ethereum MPT compact-encoded path (see "Hex-Prefix Encoding"
    in the Yellow Paper).

    The first nibble carries two flag bits: 0x20 = leaf, 0x10 = odd-length
    path. If odd, the low half of the first byte is the first path nibble.
    Remaining bytes each contribute two nibbles.
    """
    if len(compact) == 0:
        raise Exception("empty compact-encoded path")
    first = compact[0]
    is_leaf = (first & 0x20) != 0
    is_odd = (first & 0x10) != 0
    nibbles: List[int] = []
    if is_odd:
        nibbles.append(first & 0x0F)
    for b in compact[1:]:
        nibbles.append((b >> 4) & 0x0F)
        nibbles.append(b & 0x0F)
    return nibbles, is_leaf


def _mpt_lookup(
    state_root: Hash32,
    key: bytes,
    node_index: Dict[Hash32, bytes],
) -> Optional[bytes]:
    """
    Walk the Ethereum MPT rooted at `state_root` along the path
    `keccak256(key)` and return the leaf value bytes, or None on proof of
    absence (path diverges or terminates at an empty branch slot).

    Raises if a referenced node is missing from `node_index` (the witness
    pool does not cover the path) or if the proof is malformed.

    Inline children (nodes whose RLP is < 32 bytes and stored inline rather
    than referenced by hash) are not supported by this reference verifier;
    they are vanishingly rare in real-world account/storage tries.
    """
    if state_root == EMPTY_TRIE_ROOT_HASH:
        return None

    path = _bytes_to_nibbles(keccak256(key))
    path_index = 0
    current_hash = state_root

    while True:
        if current_hash not in node_index:
            raise Exception(
                f"missing MPT node {bytes(current_hash).hex()} from witness pool"
            )
        decoded = rlp.decode(node_index[current_hash])
        if not isinstance(decoded, list):
            raise Exception("MPT node RLP must decode to a list")

        if len(decoded) == 17:
            # Branch node: 16 child slots + value-at-this-prefix.
            if path_index == len(path):
                value = bytes(decoded[16])
                return value if len(value) > 0 else None
            child = bytes(decoded[path[path_index]])
            path_index += 1
            if len(child) == 0:
                return None  # empty branch slot => proof of absence
            if len(child) != 32:
                raise Exception("inline child node not supported in this reference")
            current_hash = Hash32(child)
            continue

        if len(decoded) == 2:
            # Extension or leaf node, distinguished by the leaf flag bit.
            node_nibbles, is_leaf = _compact_to_nibbles(bytes(decoded[0]))
            remaining = path[path_index:]
            if (
                len(remaining) < len(node_nibbles)
                or remaining[: len(node_nibbles)] != node_nibbles
            ):
                return None
            path_index += len(node_nibbles)
            if is_leaf:
                return bytes(decoded[1]) if path_index == len(path) else None
            child = bytes(decoded[1])
            if len(child) != 32:
                raise Exception("inline child node not supported in this reference")
            current_hash = Hash32(child)
            continue

        raise Exception(f"unexpected MPT node arity {len(decoded)}")


def _decode_account_leaf(leaf_rlp: bytes) -> Tuple[Account, Hash32]:
    """
    Decode an account-trie leaf value (`RLP([nonce, balance, storageRoot,
    codeHash])`) into the pair `(Account, storage_root)`. `Account` carries
    nonce/balance/code_hash; `storage_root` is returned separately because
    the in-memory `Account` dataclass does not expose it (storage is
    materialized as a per-account map in execution-specs).
    """
    decoded = rlp.decode(leaf_rlp)
    if not isinstance(decoded, list) or len(decoded) != 4:
        raise Exception("account leaf must be RLP([nonce, balance, storageRoot, codeHash])")
    nonce_b, balance_b, storage_root_b, code_hash_b = (bytes(x) for x in decoded)
    nonce = Uint(int.from_bytes(nonce_b, "big"))
    balance = U256(int.from_bytes(balance_b, "big"))
    storage_root = Hash32(storage_root_b.rjust(32, b"\x00"))
    code_hash = Bytes32(code_hash_b.rjust(32, b"\x00"))
    return Account(nonce=nonce, balance=balance, code_hash=code_hash), storage_root


@dataclass
class L2State:
    """
    Read-only view of the L2 state at a particular state root.

    Backed by the proof-range `ExecutionWitness` payload that the guest
    already receives as private input: the aggregated MPT node pool
    (`witnesses[*].state` plus any extra paths the witness producer
    included for boundary reads) supports `account()` and `storage()`
    via inclusion-proof verification against `state_root`, and the codes
    pool (`witnesses[*].codes`) supports `code()` lookups by `code_hash`.

    The production guest treats this as the EVM Database interface
    (zesu-style: `basic`, `storage`, `codeByHash`). This Python reference
    implements the MPT walk explicitly so the data flow is checkable end
    to end against a fixture.

    PRECOMPILE (production guest): keccak256.
        The MPT walk implemented below by `_mpt_lookup` invokes keccak256
        once per node (to look up by hash in `_node_index`) — in the
        production guest each of these is a zkVM-native primitive call.
        The walk logic itself (RLP-decode the node, classify branch /
        extension / leaf, follow path nibbles) runs as ordinary in-guest
        code on top of that primitive.
    """
    state_root: Hash32
    witnesses: Sequence["ExecutionWitness"]

    @cached_property
    def _node_index(self) -> Dict[Hash32, bytes]:
        return _build_node_index(self.witnesses)

    @cached_property
    def _code_index(self) -> Dict[Hash32, bytes]:
        return _build_code_index(self.witnesses)

    def _account_with_storage_root(self, address: Address) -> Optional[Tuple[Account, Hash32]]:
        leaf = _mpt_lookup(self.state_root, bytes(address), self._node_index)
        return None if leaf is None else _decode_account_leaf(leaf)

    def account(self, address: Address) -> Optional[Account]:
        """
        Read the account at `address`. Returns None if the account is
        absent (its absence proven by the MPT walk).
        """
        result = self._account_with_storage_root(address)
        return None if result is None else result[0]

    def storage(self, address: Address, slot: Bytes32) -> Bytes32:
        """
        Read storage[`slot`] of `address`. Returns the zero `Bytes32` if
        the account is absent or the slot is unset. Internally: look up
        the account leaf for `storage_root`, then walk the per-account
        storage trie at `keccak256(slot)`.

        Raises on a malformed leaf (RLP decoding that does not yield raw
        bytes is not a valid Ethereum storage slot — silently treating it
        as zero would mask witness tampering, so the proof must reject
        the read outright).
        """
        result = self._account_with_storage_root(address)
        if result is None:
            return Bytes32(b"\x00" * 32)
        _, storage_root = result
        slot_leaf = _mpt_lookup(storage_root, bytes(slot), self._node_index)
        if slot_leaf is None:
            return Bytes32(b"\x00" * 32)
        # Storage values are RLP(value) where `value` is the big-endian
        # integer with leading zeros stripped. Left-pad back to 32 bytes.
        decoded = rlp.decode(slot_leaf)
        if not isinstance(decoded, (bytes, bytearray)):
            raise Exception(
                f"malformed storage leaf at {bytes(address).hex()}/{bytes(slot).hex()}: "
                f"RLP decode produced {type(decoded).__name__}, expected bytes"
            )
        return Bytes32(bytes(decoded).rjust(32, b"\x00"))

    def code(self, code_hash: Hash32) -> Bytes:
        """
        Read contract code by hash from `witnesses[*].codes`. Returns
        empty bytes if the code is absent from the pool (caller is
        expected to handle that case — e.g. EOAs whose codeHash is
        `EMPTY_CODE_HASH`).
        """
        return Bytes(self._code_index.get(code_hash, b""))


def materialize_blockchain_from_execution_witness(
    chain_id: U64,
    parent_header: Header,
    execution_witness: ExecutionWitness,
) -> BlockChain:
    """
    Build the transient execution-spec `BlockChain` adapter from the stateless
    witness.

    This adapter is not a prover input. The production guest reconstructs the
    same execution context from `execution_witness.state`, `codes`, `keys`, and
    `headers`.
    """
    raise NotImplementedError("stateless execution witness replay is not implemented in this Python reference")


def state_transition_modified(
    chain_id: U64,
    parent_header: Header,
    execution_witness: ExecutionWitness,
    block: Block,
    block_rlp: bytes,
) -> vm.BlockOutput:
    """
    This function mirrors [ethereum.forks.osaka.fork.state_transition] but takes
    the protocol witness shape. It materializes the execution-spec `BlockChain`
    adapter internally and returns `block_output` so the caller can inspect logs.

    PRECOMPILE-INTENSIVE (production guest):
        This is the most precompile-heavy step in the entire proof stack.
        Replaying the block exercises (at minimum):
          - keccak256                — block-hash, MPT node hashes, tx hashes
          - secp256k1 ecrecover      — every external tx (one per signer)
          - sha256 / ripemd160       — precompiled contracts 0x02 / 0x03
          - modexp                   — precompile 0x05
          - BN254 add/mul/pairing    — precompiles 0x06 / 0x07 / 0x08
          - BLS12-381 ops            — EIP-2537 precompiles
          - KZG point evaluation     — precompile 0x0A (EIP-4844)
          - MPT verification         — every SLOAD / SSTORE / account touch
        Each of these is a zkVM-native primitive in the production guest;
        the Python reference defers to execution-specs' Python implementation,
        which in turn would compile to those primitives in a real build.
    """
    if len(block_rlp) > MAX_RLP_BLOCK_SIZE:
        raise InvalidBlock("Block rlp size exceeds MAX_RLP_BLOCK_SIZE")

    chain = materialize_blockchain_from_execution_witness(
        chain_id,
        parent_header,
        execution_witness,
    )
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
