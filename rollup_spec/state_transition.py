from dataclasses import dataclass, field
from functools import cached_property
from typing import Dict, List, Optional, Sequence, Tuple

from ethereum.crypto.hash import Hash32, keccak256
from ethereum.state import Account, Address
from ethereum_rlp import rlp
from ethereum_types.bytes import Bytes32
from ethereum_types.numeric import U256, Uint

from .block import StatelessInput
from .fork import Log


@dataclass
class ExecutionWitness:
    """
    Decoded execution witness for one payload (part of the parsed
    `StatelessInput`).

    `headers` are RLP ancestor headers ordered by block number, ending at the
    payload parent. `keys` is not in the SSZ wire format (JSON/debug only),
    defaulted empty.

    The pool must cover every account/slot read — both what block execution
    touches (served by the underlying engine) and the Linea-extra reads
    (L1->L2 rolling-hash slots at the boundary roots, FTX-sender accounts for
    §6.5 'Invalid').
    """
    state: List[bytes]
    codes: List[bytes]
    headers: List[bytes]
    keys: List[bytes] = field(default_factory=list)


@dataclass
class StatelessExecutionResult:
    """
    Result of executing one stateless input (see `execute_stateless_input`).

    Mirrors the proof output an underlying engine exposes; the Linea layer
    consumes these without re-running execution.
    """
    pre_state_root: Hash32     # parent (pre-execution) state root of the block
    post_state_root: Hash32    # post-execution state root (== executionPayload.stateRoot)
    block_logs: List[Log]      # ordered logs emitted by the block, for L2->L1 message extraction


def execute_stateless_input(stateless_input: StatelessInput) -> StatelessExecutionResult:
    """
    Validate and execute one block from its parsed `StatelessInput`.

    This is the spec's boundary to the underlying stateless block-execution
    engine (for example a Zesu-style guest, imported as a dependency or
    reimplemented). The spec does not model its internals; an underlying
    implementation is expected to perform, from the parsed `StatelessInput`
    alone:

      - witness header-chain validation and parent-header anchoring (the last
        witness header hash must equal `executionPayload.parentHash`);
      - full Engine-API payload validation (block hash, versioned hashes,
        `parentBeaconBlockRoot`, base-fee correctness, gas / blob-gas, timestamp
        vs parent, state root, receipts root, logs bloom, EIP-7928 block access
        list);
      - the EVM state transition itself.

    It returns the boundary state roots and the block logs the Linea layer builds
    on. It does NOT enforce Linea policy (e.g. empty `executionRequests`) or
    conflation-level invariants — those live in `run_l2_execution_guest`.
    """
    raise NotImplementedError(
        "stateless block execution is provided by the underlying engine "
        "(e.g. Zesu); the spec models only the Linea-specific logic on top"
    )


# ─── Witness-backed MPT state reads ────────────────────────────────────────────
#
# The Linea layer reads a little L2 state on top of delegated block execution
# (L1->L2 bridge rolling hash, FTX-sender accounts). Those reads are proven
# against a state root by walking the witness node pool directly, so the spec
# stays self-contained and checkable against a fixture rather than depending on
# the engine to expose its internal state database.

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


def _bytes_to_nibbles(data: bytes) -> List[int]:
    """Expand a byte string into a flat list of 4-bit nibbles, high first."""
    out: List[int] = []
    for b in data:
        out.append((b >> 4) & 0x0F)
        out.append(b & 0x0F)
    return out


def _compact_to_nibbles(compact: bytes) -> Tuple[List[int], bool]:
    """
    Decode the Ethereum MPT compact-encoded path (Yellow Paper "Hex-Prefix
    Encoding"). The first nibble carries two flags: 0x20 = leaf, 0x10 = odd
    length. If odd, the low half of the first byte is the first path nibble.
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
    absence (the path diverges or terminates at an empty branch slot).

    Raises if a referenced node is missing from `node_index` (the witness pool
    does not cover the path) or if the proof is malformed. Inline children
    (nodes whose RLP is < 32 bytes, stored inline rather than by hash) are not
    supported by this reference verifier; they are vanishingly rare in
    real-world account/storage tries.
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
    Decode an account-trie leaf (`RLP([nonce, balance, storageRoot, codeHash])`)
    into `(Account, storage_root)`. `storage_root` is returned separately
    because the in-memory `Account` does not expose it (execution-specs
    materializes storage as a per-account map).
    """
    decoded = rlp.decode(leaf_rlp)
    if not isinstance(decoded, list) or len(decoded) != 4:
        raise Exception("account leaf must be RLP([nonce, balance, storageRoot, codeHash])")
    nonce_b, balance_b, storage_root_b, code_hash_b = (bytes(x) for x in decoded)
    nonce = Uint(int.from_bytes(nonce_b, "big"))
    balance = U256(int.from_bytes(balance_b, "big"))
    storage_root = Hash32(storage_root_b.rjust(32, b"\x00"))
    code_hash = Hash32(code_hash_b.rjust(32, b"\x00"))
    return Account(nonce=nonce, balance=balance, code_hash=code_hash), storage_root


@dataclass
class L2State:
    """
    Read-only, witness-backed view of L2 state at a state root.

    Exposes the EVM state reads the Linea layer needs on top of delegated block
    execution: the L1->L2 bridge rolling hash (`storage`) and forced-tx sender
    accounts (`account`). Both are served by an explicit MPT inclusion walk over
    the witness node pool (`witnesses[*].state`) against `state_root`. The
    production guest performs the same walk on top of its EVM state-database
    interface (e.g. Zesu's). keccak256 (one call per node, to resolve the next
    node by hash) is a zkVM-native primitive there; the walk itself is ordinary
    in-guest code.
    """
    state_root: Hash32
    witnesses: Sequence["ExecutionWitness"]

    @cached_property
    def _node_index(self) -> Dict[Hash32, bytes]:
        return _build_node_index(self.witnesses)

    def _account_with_storage_root(self, address: Address) -> Optional[Tuple[Account, Hash32]]:
        leaf = _mpt_lookup(self.state_root, bytes(address), self._node_index)
        return None if leaf is None else _decode_account_leaf(leaf)

    def account(self, address: Address) -> Optional[Account]:
        """Account at `address`, or None if absent (proven by the MPT walk)."""
        result = self._account_with_storage_root(address)
        return None if result is None else result[0]

    def storage(self, address: Address, slot: Bytes32) -> Bytes32:
        """
        Storage value at (`address`, `slot`), or zero if the account is absent
        or the slot is unset. Looks up the account leaf for its `storage_root`,
        then walks the per-account storage trie at `keccak256(slot)`.

        Raises on a malformed leaf: RLP that does not decode to raw bytes is not
        a valid storage slot, and silently zeroing it would mask witness
        tampering, so the read is rejected outright.
        """
        result = self._account_with_storage_root(address)
        if result is None:
            return Bytes32(b"\x00" * 32)
        _, storage_root = result
        slot_leaf = _mpt_lookup(storage_root, bytes(slot), self._node_index)
        if slot_leaf is None:
            return Bytes32(b"\x00" * 32)
        # Storage values are RLP(value) where `value` is the big-endian integer
        # with leading zeros stripped. Left-pad back to 32 bytes.
        decoded = rlp.decode(slot_leaf)
        if not isinstance(decoded, (bytes, bytearray)):
            raise Exception(
                f"malformed storage leaf at {bytes(address).hex()}/{bytes(slot).hex()}: "
                f"RLP decode produced {type(decoded).__name__}, expected bytes"
            )
        return Bytes32(bytes(decoded).rjust(32, b"\x00"))
