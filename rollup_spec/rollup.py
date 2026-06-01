from dataclasses import dataclass, field
from functools import lru_cache
from pathlib import Path
from typing import List, Optional, Sequence, Tuple

import ckzg
import lz4.block

from ethereum.crypto.hash import Hash32, keccak256
from ethereum.crypto.kzg import (
    KZGCommitment,
    kzg_commitment_to_versioned_hash,
)
from ethereum.forks.osaka.transactions import (
    AccessListTransaction,
    BlobTransaction,
    FeeMarketTransaction,
    LegacyTransaction,
    SetCodeTransaction,
    Transaction,
    decode_transaction,
    recover_sender,
)
from ethereum.state import Address
from ethereum_rlp import rlp
from ethereum_types.bytes import Bytes, Bytes32, Bytes48
from ethereum_types.numeric import U64, Uint

from .block import block_hash, decode_block_rlp
from .l2_execution import (
    L2ExecutionProof,
    L2ExecutionProofPublicInput,
    hash_address_list,
    hash_hash_list,
)

L2_L1_TREE_DEPTH = 5
ZERO_HASH32 = Hash32(b"\x00" * 32)

# EIP-4844 blob size: FIELD_ELEMENTS_PER_BLOB (4096) × BYTES_PER_FIELD_ELEMENT (32).
# The KZG commitment is computed over a polynomial defined by exactly this many
# evaluations, so the byte payload handed to `ckzg.verify_blob_kzg_proof` must
# be exactly `BLOB_BYTES_LENGTH` bytes — shorter compressed output is zero-padded.
BLOB_BYTES_LENGTH = 4096 * 32

# EIP-4844 trusted setup (4096 G1 + 65 G2 monomial points from the Ethereum
# KZG ceremony). The `ckzg` wheel does not bundle a setup file, so we reuse
# the one already vendored in this repo for the hardhat contract tests. All
# four trusted-setup files we found in linea-monorepo (`contracts/test/...`,
# `contracts/scripts/testEIP4844/...`, `tmp/besu-eth/.../resources/...`)
# produce identical KZG commitments; the byte-level sha256 differences are
# just file-format ordering variants that ckzg parses transparently.
_TRUSTED_SETUP_PATH = (
    Path(__file__).resolve().parents[1]
    / "contracts" / "test" / "hardhat" / "_testData" / "trusted_setup.txt"
)


@lru_cache(maxsize=1)
def _trusted_setup():
    """
    Load the EIP-4844 trusted setup once on first use; cached for the
    process lifetime via `lru_cache`. `precompute=0` skips the optional
    FK20 multi-scalar-multiplication precomputation that only matters for
    `compute_cells_and_kzg_proofs`; we don't need it for the single
    polynomial evaluation we perform per blob.
    """
    if not _TRUSTED_SETUP_PATH.is_file():
        raise FileNotFoundError(
            f"trusted setup not found at {_TRUSTED_SETUP_PATH}. "
            "The Python reference expects the linea-monorepo layout — re-fetch via "
            "`curl -L https://raw.githubusercontent.com/ethereum/c-kzg-4844/main/src/trusted_setup.txt "
            f"-o {_TRUSTED_SETUP_PATH}` if missing."
        )
    return ckzg.load_trusted_setup(str(_TRUSTED_SETUP_PATH), 0)


@dataclass
class TruncatedEthereumBlock:
    """
    TruncatedEthereumBlock is the truncated content of an Ethereum block as it
    appears in the DA blob payload (§3.2). The rollup guest cross-checks
    `froms` against each l2-execution proof's `txFromsHash` (§2.2 step 3) and
    `block_hash` against the l2-execution proofs' `endBlockHash` at boundaries
    (§2.2 step 5); no separate full-block comparison is required.
    """
    timestamp: U64
    block_hash: Hash32
    prev_randao: Bytes32
    transactions: List[bytes]
    froms: List[Address]


@dataclass
class ShnarfWitness:
    """
    ShnarfWitness is the preimage of a shnarf (§3.1):
    `Hash(parentShnarf, lastBlockHash, blobHash)`.
    """
    parent_shnarf: Hash32
    last_block_hash: Hash32
    blob_hash: Hash32

    def hash(self) -> Hash32:
        return keccak256(self.parent_shnarf + self.last_block_hash + self.blob_hash)


@dataclass
class BlobWitness:
    """
    Per-blob witness for the rollup proof.

    Fields:
      - `block_number_range`: `(startBlockNumber, endBlockNumber)` of the
        L2 blocks contained in this blob.
      - `block_rlps`: the canonical full block RLPs (same shape the
        l2-execution proof receives — header + tx list [+ withdrawals], EIP-2718
        typed transactions in full signed form), one per block in
        `block_number_range`. Truncation per §3.2 happens *inside* the
        guest from these full RLPs; there is no separately-witnessed
        truncated form.
      - `blob_hash`: the L1-anchored versioned hash of the DA blob. The
        rollup guest computes the KZG commitment from the computed padded
        blob bytes and checks its versioned hash against this value.
      - `blob_kzg_proof`: KZG proof for the computed blob bytes and computed
        commitment.

    The proof statement (§2.2 step 1) is: the guest computes the
    blob bytes from `block_rlps` (truncate → RLP-encode → LZ4-compress →
    zero-pad), computes the KZG commitment from those bytes, checks
    `kzg_commitment_to_versioned_hash(computed_commitment) == blob_hash`,
    then runs `ckzg.verify_blob_kzg_proof` on the computed tuple. If the
    sequencer committed to anything other than this exact byte sequence,
    the versioned-hash or KZG checks reject — so there is no separate
    byte-equality check. The truncated blocks themselves are an internal-only
    intermediate; downstream consumers (block-hash boundary checks in
    §2.2 step 5, sender-list cross-checks in step 3) take the computed
    truncated form, not a witnessed copy.
    """
    block_number_range: tuple[int, int]
    block_rlps: List[bytes]
    blob_hash: Hash32
    blob_kzg_proof: Bytes48

    def verify_blob(
        self,
        chain_id: U64,
    ) -> Tuple[Hash32, List["TruncatedEthereumBlock"], List[Hash32]]:
        """
        Recompute the blob payload from `block_rlps`, compute its KZG
        commitment, and verify that it binds the bytes to the declared
        `blob_hash`
        (§2.2 step 1). This single check subsumes the older
        compression-equality assertion: the KZG verifier accepts iff the
        bytes the guest computed are identical to the bytes the sequencer
        committed to on L1.

        Steps:
          1. Decode each `block_rlps[i]` and apply the canonical DA
             truncation rule (§3.2), capturing the header's `parent_hash`
             alongside.
          2. Serialize the truncated list via `rlp_encode_truncated_blocks`,
             LZ4-compress (raw block format, no length header) via
             `compress_lz4`, and zero-pad to `BLOB_BYTES_LENGTH`.
          3. Compute the KZG commitment from the padded bytes, derive its
             versioned hash, and assert it equals the L1-anchored `blob_hash`.
          4. Delegate the full EIP-4844 verification (Fiat-Shamir challenge
             derivation, polynomial evaluation in Lagrange form, pairing
             check) to `ckzg.verify_blob_kzg_proof` with the computed
             commitment — the same canonical entry point consensus-level
             clients call. The production guest runs the equivalent in-zkVM
             primitive or deterministic linked implementation with fixed
             trusted-setup semantics; this reference defers to the c-kzg-4844
             binding rather than re-deriving its internals.

        Returns `(blob_hash, truncated, parent_hashes)`:
          - `blob_hash`: the versioned hash committed to on L1.
          - `truncated`: the computed `TruncatedEthereumBlock` per block,
            consumed by downstream steps (block-hash boundary alignment,
            sender-list cross-check).
          - `parent_hashes`: each block's `header.parent_hash`. Exposed
            so `run_rollup_guest` can verify the full block-hash chain
            (§2.2 step 5): every block's claimed parent must match the
            previous block's computed hash, anchored at the first
            l2-execution proof's `parentBlockHash` and at each l2-execution
            proof's `endBlockHash` boundary. Without this, intermediate
            blocks inside an l2-execution range would only be bound
            transitively, leaving room for a malicious prover to swap a
            non-boundary block as long as its successor's `parent_hash`
            still pointed to the *original* (un-swapped) block.

        `chain_id` is required to recover transaction senders during the
        truncation.
        """
        truncated: List["TruncatedEthereumBlock"] = []
        parent_hashes: List[Hash32] = []
        for rlp_bytes in self.block_rlps:
            # Decode once to capture the header's parent_hash; the
            # downstream `truncate_block_rlp` redoes the decode — small
            # redundancy that keeps the reference implementation simple.
            header = decode_block_rlp(rlp_bytes).header
            parent_hashes.append(Hash32(header.parent_hash))
            truncated.append(truncate_block_rlp(rlp_bytes, chain_id))

        serialized = rlp_encode_truncated_blocks(truncated)
        compressed = compress_lz4(serialized)
        if len(compressed) > BLOB_BYTES_LENGTH:
            raise Exception(
                f"compressed blob payload {len(compressed)} bytes exceeds "
                f"EIP-4844 blob size {BLOB_BYTES_LENGTH}"
            )
        # Pad with zero bytes to fill the full EIP-4844 blob payload. The
        # KZG commitment is taken over a polynomial of exactly
        # FIELD_ELEMENTS_PER_BLOB evaluations; the sequencer pads with
        # zeros and so must the guest for the commitment to match.
        padded = compressed + b"\x00" * (BLOB_BYTES_LENGTH - len(compressed))

        setup = _trusted_setup()
        try:
            # The commitment is not witnessed. It is recomputed inside the
            # rollup guest from the exact padded payload the guest derived.
            blob_kzg_commitment = KZGCommitment(
                ckzg.blob_to_kzg_commitment(padded, setup),
            )
        except Exception as exc:
            raise Exception("invalid blob KZG commitment computation") from exc

        blob_hash = Hash32(kzg_commitment_to_versioned_hash(blob_kzg_commitment))
        if blob_hash != self.blob_hash:
            raise Exception("computed blob KZG commitment does not match blobHash")

        try:
            # ┌─ PRECOMPILE (production guest): BLS12-381 / KZG verifier ─────┐
            # │ The zkVM exposes EIP-4844 blob commitment / verification as   │
            # │ native primitives or a deterministic linked implementation.   │
            # │ These calls hide BLS12-381 multi-scalar multiplication,       │
            # │ polynomial evaluation in Lagrange form, and pairing checks.   │
            # │                                                               │
            # │ Soundness of the compression statement comes from computing   │
            # │ the commitment from `padded`, matching its versioned hash to  │
            # │ L1's `blobHash`, and verifying the proof against that         │
            # │ computed commitment.                                          │
            # └───────────────────────────────────────────────────────────────┘
            ok = ckzg.verify_blob_kzg_proof(
                padded,
                blob_kzg_commitment,
                self.blob_kzg_proof,
                setup,
            )
        except Exception as exc:
            # Malformed witness bytes (invalid G1 encoding, blob length
            # mismatch, field-element overflow, …) raise from ckzg; surface
            # them as a proof-verification failure with the original cause.
            raise Exception("invalid KZG proof") from exc
        if not ok:
            raise Exception("invalid KZG proof")

        return blob_hash, truncated, parent_hashes


@dataclass
class RollupPublicInput:
    """
    The 14-field rollup / rollup-aggregation public input tuple from
    Readme.md section 2.4.
    """
    end_block_number: U64
    end_block_timestamp: U64
    l2_l1_bridge_transaction_tree: Hash32
    parent_l1_l2_bridge_rolling_hash: Hash32
    parent_l1_l2_bridge_rolling_hash_message_number: U64
    end_l1_l2_bridge_rolling_hash: Hash32
    end_l1_l2_bridge_rolling_hash_message_number: U64
    dynamic_chain_config_hash: Hash32
    parent_ftx_rolling_hash: Hash32
    end_ftx_rolling_hash: Hash32
    last_processed_ftx_number: U64
    filtered_addresses_hash: Hash32
    parent_shnarf: Hash32
    end_shnarf: Hash32


@dataclass
class RollupProofPrivateInput:
    """
    Logical rollup request. One rollup proof folds K >= 1 DA blobs and N >= 1
    l2-execution proofs tiling the combined block range.

    `chain_id` is needed for sender recovery during DA truncation (§2.2
    step 2). It is committed transitively via the l2-execution proofs'
    `dynamicChainConfigHash` PI field — `assert_l2_execution_continuity`
    (step 8) ensures the same value flows across all l2-execution proofs in
    the rollup range, so the rollup proof inherits chain-config integrity
    from the l2-execution proofs it recursively verifies.
    """
    parent_shnarf: Hash32
    chain_id: U64
    blobs: List[BlobWitness]
    l2_execution_proofs: List[L2ExecutionProof]


@dataclass
class RollupProof:
    """
    Reference wrapper for a rollup proof plus the root/address preimages
    consumed by the rollup-aggregation proof. `proof` stands in for recursive
    STARK bytes.
    """
    public_inputs: RollupPublicInput
    start_block_number: U64
    end_block_number: U64
    proof: bytes = b""
    l2_l1_roots: List[Hash32] = field(default_factory=list)
    filtered_addresses: List[Address] = field(default_factory=list)


def run_rollup_guest(rollup_input: RollupProofPrivateInput) -> RollupProof:
    """
    rollup: per blob, computes the canonical compressed payload from
    `block_rlps` (truncate → RLP-encode → LZ4-compress → zero-pad to
    BLOB_BYTES_LENGTH), computes the KZG commitment from those bytes, checks
    it against the L1-anchored `blobHash`, and runs `ckzg.verify_blob_kzg_proof`
    on the computed commitment. Recursively verifies the N l2-execution proofs,
    checks continuity, builds the L2->L1 Merkle-root commitment, collects FTX
    outputs, and emits the 14-field rollup PI tuple (§2.4).
    """
    if len(rollup_input.blobs) == 0:
        raise Exception("rollup proof must cover at least one blob")
    if len(rollup_input.l2_execution_proofs) == 0:
        raise Exception("rollup proof must consume at least one l2-execution proof")

    current_shnarf = rollup_input.parent_shnarf
    truncated_blocks: List[TruncatedEthereumBlock] = []
    parent_hashes: List[Hash32] = []
    expected_blob_start: Optional[int] = None

    for blob in rollup_input.blobs:
        # KZG binding on the *computed* payload (§2.2 step 1): the guest
        # recomputes the compressed blob bytes from the witnessed full
        # block RLPs, computes the KZG commitment, matches the derived
        # versioned hash to L1's `blobHash`, and verifies the KZG proof
        # against the computed commitment. Downstream
        # cross-checks anchor the truncated content to the l2-execution proofs.
        blob_hash, blob_blocks, blob_parent_hashes = blob.verify_blob(rollup_input.chain_id)
        start_block_number, end_block_number = blob.block_number_range
        expected_block_count = end_block_number + 1 - start_block_number
        if len(blob_blocks) != expected_block_count:
            raise Exception("blob block range is inconsistent with witnessed truncated blocks")
        if len(blob_blocks) == 0:
            raise Exception("rollup proof cannot include an empty blob block range")
        if expected_blob_start is not None and start_block_number != expected_blob_start:
            raise Exception("blob ranges must be contiguous")

        truncated_blocks.extend(blob_blocks)
        parent_hashes.extend(blob_parent_hashes)
        current_shnarf = ShnarfWitness(
            current_shnarf,
            blob_blocks[-1].block_hash,
            blob_hash,
        ).hash()
        expected_blob_start = end_block_number + 1

    blob_start_block_number = rollup_input.blobs[0].block_number_range[0]
    blob_end_block_number = rollup_input.blobs[-1].block_number_range[1]
    verify_l2_execution_proof_tiling(
        rollup_input.l2_execution_proofs,
        blob_start_block_number,
        blob_end_block_number,
    )

    concatenated_froms: List[Address] = []
    concatenated_l2_l1_messages: List[Hash32] = []
    concatenated_filtered_addresses: List[Address] = []
    truncated_froms: List[Address] = []
    truncated_block_hashes = [block.block_hash for block in truncated_blocks]

    for proof in rollup_input.l2_execution_proofs:
        verify_l2_execution_proof(proof)
        concatenated_froms.extend(proof.tx_froms)
        concatenated_l2_l1_messages.extend(proof.l2_l1_messages)
        concatenated_filtered_addresses.extend(proof.filtered_addresses)

    for block in truncated_blocks:
        truncated_froms.extend(block.froms)

    if concatenated_froms != truncated_froms:
        raise Exception("l2-execution proof txFroms do not match blob blockData.froms")

    first_proof = rollup_input.l2_execution_proofs[0]
    last_proof = rollup_input.l2_execution_proofs[-1]
    for proof in rollup_input.l2_execution_proofs:
        boundary_index = int(proof.end_block_number) - blob_start_block_number
        if boundary_index < 0 or boundary_index >= len(truncated_block_hashes):
            raise Exception("l2-execution proof boundary falls outside the blob block range")
        if proof.public_inputs.end_block_hash != truncated_block_hashes[boundary_index]:
            raise Exception("l2-execution proof end block hash does not match blob data at its boundary")

    # Parent-hash continuity across the *entire* block range (§2.2 step 5):
    # without this every block strictly between l2-execution-proof boundaries
    # would only be bound transitively through `from`-list matching, which
    # accepts header changes that don't touch transactions (e.g., timestamp
    # or prevRandao swaps).
    #
    # The first block's parent must equal the first l2-execution proof's
    # `parentBlockHash`; every subsequent block's parent must equal the
    # previous block's computed hash. Combined with the boundary check
    # above, this anchors every block to the chain that the l2-execution
    # proofs verified.
    if parent_hashes[0] != first_proof.public_inputs.parent_block_hash:
        raise Exception(
            "blob's first block does not descend from the first l2-execution proof's parentBlockHash"
        )
    for i in range(1, len(parent_hashes)):
        if parent_hashes[i] != truncated_block_hashes[i - 1]:
            raise Exception(
                f"blob block-hash chain breaks at index {i}: "
                f"parent_hash != previous block's hash"
            )

    for left, right in zip(rollup_input.l2_execution_proofs, rollup_input.l2_execution_proofs[1:]):
        assert_l2_execution_continuity(left.public_inputs, right.public_inputs)

    l2_l1_roots, l2_l1_bridge_transaction_tree = build_l2_messages_tree(
        concatenated_l2_l1_messages,
    )
    public_inputs = RollupPublicInput(
        end_block_number=last_proof.public_inputs.end_block_number,
        end_block_timestamp=last_proof.public_inputs.end_block_timestamp,
        l2_l1_bridge_transaction_tree=l2_l1_bridge_transaction_tree,
        parent_l1_l2_bridge_rolling_hash=first_proof.public_inputs.parent_l1_l2_bridge_rolling_hash,
        parent_l1_l2_bridge_rolling_hash_message_number=(
            first_proof.public_inputs.parent_l1_l2_bridge_rolling_hash_message_number
        ),
        end_l1_l2_bridge_rolling_hash=last_proof.public_inputs.end_l1_l2_bridge_rolling_hash,
        end_l1_l2_bridge_rolling_hash_message_number=(
            last_proof.public_inputs.end_l1_l2_bridge_rolling_hash_message_number
        ),
        dynamic_chain_config_hash=first_proof.public_inputs.dynamic_chain_config_hash,
        parent_ftx_rolling_hash=first_proof.public_inputs.parent_ftx_rolling_hash,
        end_ftx_rolling_hash=last_proof.public_inputs.end_ftx_rolling_hash,
        last_processed_ftx_number=last_proof.public_inputs.last_processed_ftx_number,
        filtered_addresses_hash=hash_address_list(concatenated_filtered_addresses),
        parent_shnarf=rollup_input.parent_shnarf,
        end_shnarf=current_shnarf,
    )

    return RollupProof(
        public_inputs=public_inputs,
        start_block_number=blob_start_block_number,
        end_block_number=blob_end_block_number,
        l2_l1_roots=l2_l1_roots,
        filtered_addresses=concatenated_filtered_addresses,
    )


def verify_l2_execution_proof(proof: L2ExecutionProof) -> None:
    """
    Verify an inner l2-execution proof against its claimed public inputs.

    PRECOMPILE (production guest): recursive STARK verification.
        The zkVM exposes the inner-proof verifier as a circuit primitive
        (typically wired through the underlying field's hash precompile,
        e.g. Poseidon2 for KoalaBear / Goldilocks). In this reference,
        the recursive-verify step is implicit — `L2ExecutionProof.proof`
        stands in for the recursive STARK bytes the guest would actually
        check. The Python reference only re-checks the hash-preimage
        bindings (`txFromsHash`, `L2L1MessagesHash`, `filteredAddressesHash`)
        the rollup proof consumes alongside the PI tuple.
    """
    if proof.public_inputs.end_block_number != proof.end_block_number:
        raise Exception("l2-execution proof range metadata does not match public inputs")
    # The three checks below are PRECOMPILE: keccak256 in production (used
    # to verify the preimage bindings that the rollup proof consumes).
    if hash_hash_list(proof.l2_l1_messages) != proof.public_inputs.l2_l1_messages_hash:
        raise Exception("invalid L2-to-L1 message-list preimage")
    if hash_address_list(proof.tx_froms) != proof.public_inputs.tx_froms_hash:
        raise Exception("invalid txFromsHash preimage")
    if hash_address_list(proof.filtered_addresses) != proof.public_inputs.filtered_addresses_hash:
        raise Exception("invalid l2-execution filteredAddressesHash preimage")


def verify_l2_execution_proof_tiling(
    l2_execution_proofs: Sequence[L2ExecutionProof],
    start_block_number: int,
    end_block_number: int,
) -> None:
    expected_start = start_block_number
    for proof in l2_execution_proofs:
        if proof.start_block_number != expected_start:
            raise Exception("l2-execution proofs do not tile the blob block range")
        expected_start = proof.end_block_number + 1
    if expected_start != end_block_number + 1:
        raise Exception("l2-execution proofs do not cover the full blob block range")


def assert_l2_execution_continuity(
    left: L2ExecutionProofPublicInput,
    right: L2ExecutionProofPublicInput,
) -> None:
    if left.end_block_hash != right.parent_block_hash:
        raise Exception("l2-execution block-hash continuity failed")
    if left.end_l1_l2_bridge_rolling_hash != right.parent_l1_l2_bridge_rolling_hash:
        raise Exception("l2-execution L1-to-L2 rolling-hash continuity failed")
    if left.end_l1_l2_bridge_rolling_hash_message_number != right.parent_l1_l2_bridge_rolling_hash_message_number:
        raise Exception("l2-execution L1-to-L2 rolling-hash-number continuity failed")
    if left.dynamic_chain_config_hash != right.dynamic_chain_config_hash:
        raise Exception("l2-execution dynamic chain configuration continuity failed")
    if left.end_ftx_rolling_hash != right.parent_ftx_rolling_hash:
        raise Exception("l2-execution FTX rolling-hash continuity failed")


def build_l2_messages_tree(msgs: Sequence[Hash32]) -> Tuple[List[Hash32], Hash32]:
    """
    Build L2-to-L1 message trees exactly as specified:
    - Pad the ordered message-hash list with zero Hash32 values until its
      length is a multiple of 32.
    - Split the padded list into consecutive 32-leaf chunks.
    - Merkle-hash each chunk as a complete depth-5 binary tree with keccak.
    - Flat-hash the ordered roots with keccak256(root_1 || ... || root_n).

    The returned root list is the private preimage used by aggregation and L1
    calldata; the returned hash is the public `L2L1BridgeTransactionTree`.
    """
    roots = build_l2_message_roots(msgs)
    return roots, hash_hash_list(roots)


def build_l2_message_roots(msgs: Sequence[Hash32]) -> List[Hash32]:
    leaves_per_tree = 1 << L2_L1_TREE_DEPTH
    padded_msgs = list(msgs)
    padding = (-len(padded_msgs)) % leaves_per_tree
    padded_msgs.extend(ZERO_HASH32 for _ in range(padding))

    roots = []
    for start in range(0, len(padded_msgs), leaves_per_tree):
        roots.append(
            merkle_root_fixed_depth(
                padded_msgs[start:start + leaves_per_tree],
                L2_L1_TREE_DEPTH,
            ),
        )
    return roots


def merkle_root_fixed_depth(leaves: Sequence[Hash32], depth: int) -> Hash32:
    leaf_count = 1 << depth
    if len(leaves) > leaf_count:
        raise Exception("too many leaves for fixed-depth tree")

    layer = [bytes(leaf) for leaf in leaves]
    layer.extend(bytes(ZERO_HASH32) for _ in range(leaf_count - len(layer)))
    while len(layer) > 1:
        layer = [
            keccak256(layer[i] + layer[i + 1])
            for i in range(0, len(layer), 2)
        ]
    return Hash32(layer[0])


def _signature_stripped_tx_bytes(tx: Transaction) -> bytes:
    """
    Canonical signature- and chainID-stripped tx encoding per §3.2.

    For legacy transactions: RLP([nonce, gas_price, gas, to, value, data])
    — chain_id was implicit in `v` for EIP-155, dropped here along with
    the entire `(v, r, s)` triplet.

    For typed (EIP-2718) transactions: `type_byte || RLP([...])` with
    `chain_id` (always the first field of the typed-tx RLP) and the
    `(y_parity, r, s)` signature triplet both omitted.
    """
    if isinstance(tx, LegacyTransaction):
        return rlp.encode([tx.nonce, tx.gas_price, tx.gas, tx.to, tx.value, tx.data])
    if isinstance(tx, AccessListTransaction):
        return b"\x01" + rlp.encode([
            tx.nonce, tx.gas_price, tx.gas, tx.to, tx.value, tx.data, tx.access_list,
        ])
    if isinstance(tx, FeeMarketTransaction):
        return b"\x02" + rlp.encode([
            tx.nonce, tx.max_priority_fee_per_gas, tx.max_fee_per_gas, tx.gas,
            tx.to, tx.value, tx.data, tx.access_list,
        ])
    if isinstance(tx, BlobTransaction):
        return b"\x03" + rlp.encode([
            tx.nonce, tx.max_priority_fee_per_gas, tx.max_fee_per_gas, tx.gas,
            tx.to, tx.value, tx.data, tx.access_list,
            tx.max_fee_per_blob_gas, tx.blob_versioned_hashes,
        ])
    if isinstance(tx, SetCodeTransaction):
        return b"\x04" + rlp.encode([
            tx.nonce, tx.max_priority_fee_per_gas, tx.max_fee_per_gas, tx.gas,
            tx.to, tx.value, tx.data, tx.access_list, tx.authorizations,
        ])
    raise Exception(f"unknown transaction type {type(tx).__name__}")


def truncate_block_rlp(block_rlp: bytes, chain_id: U64) -> TruncatedEthereumBlock:
    """
    Decode the canonical full block RLP and apply the §3.2 DA truncation
    rule, returning a `TruncatedEthereumBlock`.

    Header-derived fields:
      - `timestamp` and `prev_randao` are taken from the decoded header.
      - `block_hash = keccak256(rlp_encode(header))` — a Type-1 block hash
        depends on the full canonical header encoding.

    Per-transaction fields:
      - `transactions[i]` is the signature- and chainID-stripped canonical
        bytes (see `_signature_stripped_tx_bytes`).
      - `froms[i]` is the sender recovered via `recover_sender(chain_id, tx)`.
    """
    block = decode_block_rlp(block_rlp)
    bh = block_hash(block.header)

    transactions: List[bytes] = []
    froms: List[Address] = []
    for tx_item in block.transactions:
        # `Block.transactions` holds typed (EIP-2718) txs as bytes and
        # legacy txs as decoded `LegacyTransaction` objects.
        if isinstance(tx_item, (bytes, bytearray)):
            decoded_tx: Transaction = decode_transaction(Bytes(tx_item))
        else:
            decoded_tx = tx_item
        transactions.append(_signature_stripped_tx_bytes(decoded_tx))
        froms.append(recover_sender(chain_id, decoded_tx))

    return TruncatedEthereumBlock(
        timestamp=U64(block.header.timestamp),
        block_hash=bh,
        prev_randao=Bytes32(block.header.prev_randao),
        transactions=transactions,
        froms=froms,
    )


def rlp_encode_truncated_blocks(blocks: Sequence[TruncatedEthereumBlock]) -> bytes:
    """
    Canonical RLP serialization of the per-blob truncated-block list
    (§3.2). Both the sequencer (producing the blob) and the rollup
    guest (recomputing the compressed payload for KZG verification) must
    use this exact encoding — any drift causes the KZG verifier to reject.

    Layout::

      RLP([
        [
          uint(timestamp),
          bytes32(blockHash),
          bytes32(prevRandao),
          [stripped_tx_1, stripped_tx_2, ...],
          [from_1, from_2, ...],
        ],
        ...
      ])
    """
    items = [
        [
            Uint(int(b.timestamp)),
            bytes(b.block_hash),
            bytes(b.prev_randao),
            list(b.transactions),
            [bytes(f) for f in b.froms],
        ]
        for b in blocks
    ]
    return rlp.encode(items)


def compress_lz4(data: bytes) -> bytes:
    """
    LZ4-compress the canonical RLP-encoded truncated-block payload using
    the raw LZ4 block format (no 4-byte uncompressed-size header). The
    rollup guest zero-pads this output to `BLOB_BYTES_LENGTH` and
    hands the padded result to `ckzg.verify_blob_kzg_proof` (§2.2 step 1).

    The sequencer producing the blob must use the same compression mode
    (LZ4 block, `store_size=False`) and compression level — both choices
    are protocol-level decisions and must match byte-for-byte for the KZG
    verifier to accept on the L1-committed `blobHash`.

    NOT a precompile — LZ4 runs as ordinary in-guest code. A vendored C
    library (lz4) compiled into the RISC-V guest performs the compression
    in linear time; soundness comes from KZG verification on the
    computed payload (§2.2 step 1), not from the LZ4 internals.
    """
    return lz4.block.compress(data, store_size=False)
