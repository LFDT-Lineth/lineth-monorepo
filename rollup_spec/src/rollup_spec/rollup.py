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
from .fork import (
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
from ethereum_types.bytes import Bytes, Bytes32
from ethereum_types.numeric import U64, Uint

from .block import block_hash, decode_block_rlp
from .l2_execution import (
    L2ExecutionProof,
    L2ExecutionProofPublicInput,
    VerifiableL2ExecutionProof,
    hash_address_list,
    hash_hash_list,
)

L2_L1_TREE_DEPTH = 5
ZERO_HASH32 = Hash32(b"\x00" * 32)

# EIP-4844 blob size: FIELD_ELEMENTS_PER_BLOB (4096) × BYTES_PER_FIELD_ELEMENT (32).
# The KZG commitment is computed over a polynomial defined by exactly this many
# evaluations, so the byte payload handed to `ckzg.blob_to_kzg_commitment` must
# be exactly `BLOB_BYTES_LENGTH` bytes — shorter compressed output is zero-padded.
BLOB_BYTES_LENGTH = 4096 * 32

# Big-endian width of the per-conflation segment length prefix within the DA
# stream (§3.1): `[len][lz4(rlp(conflation))]`. 4 bytes comfortably bounds any
# realistic compressed conflation size well under 2**32.
SEGMENT_LENGTH_PREFIX_BYTES = 4

# EIP-4844 trusted setup (4096 G1 + 65 G2 monomial points from the Ethereum
# KZG ceremony). The `ckzg` wheel does not bundle a setup file, so we reuse
# the one already vendored in this repo for the hardhat contract tests. All
# four trusted-setup files we found in lineth-monorepo (`contracts/test/...`,
# `contracts/scripts/testEIP4844/...`, `tmp/besu-eth/.../resources/...`)
# produce identical KZG commitments; the byte-level sha256 differences are
# just file-format ordering variants that ckzg parses transparently.
# parents: [0]=src/rollup_spec, [1]=src, [2]=rollup_spec (project), [3]=repo root.
_TRUSTED_SETUP_PATH = (
    Path(__file__).resolve().parents[3]
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
            "The Python reference expects the lineth-monorepo layout — re-fetch via "
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
class DrhWitness:
    """
    DrhWitness is the preimage of a Data Rolling Hash (DRH) fold step (§3.1):
    `keccak256(parentDrh || chunkHash)`.

    The DRH is a pure DA accumulator over the ordered sequence of published
    chunks. Execution continuity — the role the old 3-input shnarf's
    `lastBlockHash` used to play — lives in the explicit `parentBlockHash` /
    `endBlockHash` public-input fields instead (§2.4): under shared chunks,
    "the last block completing in chunk i" depends on two adjacent proofs'
    witnesses, so a 1-input accumulator is the shape a single proof can
    recompute alone.
    """
    parent_drh: Hash32
    chunk_hash: Hash32

    def hash(self) -> Hash32:
        return keccak256(self.parent_drh + self.chunk_hash)


@dataclass
class ConflationWitness:
    """
    Per-conflation witness for the rollup proof (§2.2, §3.1). A conflation is
    exactly one l2-execution proof's block range; `block_rlps` are the
    canonical full block RLPs published through the DA path (header + tx list
    [+ withdrawals], EIP-2718 typed transactions in full signed form), one per
    block, paired 1:1 and in order with `RollupProofPrivateInput.l2_execution_proofs`.
    The l2-execution proof receives Engine API `NewPayloadRequest` inputs
    instead; the rollup proof cross-checks this DA material against
    l2-execution public block hashes and `txFromsHash`. Truncation per §3.2
    happens *inside* the guest from these full RLPs; there is no separately-
    witnessed truncated form.

    Each conflation is compressed INDEPENDENTLY — truncate → RLP-encode →
    LZ4-compress, one segment per conflation, length-prefixed — and the
    resulting segments are concatenated in order to form the DA byte stream
    (§3.1). LZ4 back-references never cross a segment boundary, so this proof
    can recompress its own conflations without any foreign witness data.
    """
    block_rlps: List[bytes]


def _truncate_conflation(
    block_rlps: Sequence[bytes],
    chain_id: U64,
) -> Tuple[List["TruncatedEthereumBlock"], List[Hash32]]:
    """
    Decode and truncate one conflation's block RLPs (§3.2), capturing each
    block's header `parent_hash` alongside.

    Returns `(truncated, parent_hashes)`:
      - `truncated`: the computed `TruncatedEthereumBlock` per block,
        consumed by downstream steps (block-hash boundary alignment,
        sender-list cross-check).
      - `parent_hashes`: each block's `header.parent_hash`. Exposed so
        `run_rollup_guest` can verify the full block-hash chain (§2.2 step 5):
        every block's claimed parent must match the previous block's computed
        hash, anchored at the first l2-execution proof's `parentBlockHash`
        and at each l2-execution proof's `endBlockHash` boundary. Without
        this, intermediate blocks inside an l2-execution range would only be
        bound transitively, leaving room for a malicious prover to swap a
        non-boundary block as long as its successor's `parent_hash` still
        pointed to the *original* (un-swapped) block.
    """
    truncated: List["TruncatedEthereumBlock"] = []
    parent_hashes: List[Hash32] = []
    for rlp_bytes in block_rlps:
        # Decode once to capture the header's parent_hash; the downstream
        # `truncate_block_rlp` redoes the decode — small redundancy that
        # keeps the reference implementation simple.
        header = decode_block_rlp(rlp_bytes).header
        parent_hashes.append(Hash32(header.parent_hash))
        truncated.append(truncate_block_rlp(rlp_bytes, chain_id))
    return truncated, parent_hashes


def _compress_conflation_segment(truncated: Sequence["TruncatedEthereumBlock"]) -> bytes:
    """
    Independently RLP-encode and LZ4-compress one conflation's truncated
    blocks, and prefix the result with its compressed length (§3.1):
    `[len][lz4(rlp(conflation))]`. The sequencer and the rollup guest must
    agree byte-for-byte on this framing for the KZG verifier to accept.
    """
    segment = compress_lz4(rlp_encode_truncated_blocks(truncated))
    return len(segment).to_bytes(SEGMENT_LENGTH_PREFIX_BYTES, "big") + segment


def _verify_and_fold_chunks(
    own_stream_bytes: bytes,
    start_offset: int,
    chunks: Sequence[Hash32],
    opaque_prefix_bytes: bytes,
    opaque_suffix_bytes: bytes,
    parent_drh: Hash32,
    boundary_prev_drh: Optional[Hash32],
) -> Tuple[Hash32, int]:
    """
    Slice `own_stream_bytes` (this proof's own concatenated conflation
    segments) across the chunks it touches, reconstruct each chunk's full
    published bytes, verify each chunk's KZG commitment against its anchored
    hash (`chunks[k]`), and fold the DRH chain across them (§3.1, §3.4).

    `opaque_prefix_bytes` / `opaque_suffix_bytes` are foreign bytes not owned
    by this proof — relevant only at the two ends of the touched range, never
    per-chunk: `opaque_prefix_bytes` fills the start of the FIRST touched
    chunk when `start_offset > 0`; `opaque_suffix_bytes` fills the end of the
    LAST touched chunk when this proof's data doesn't reach `chunkSize`. Both
    default to empty. The guest witnesses them as opaque bytes purely to
    reconstruct each boundary chunk's full published content for KZG
    verification — their content is never interpreted, only reproduced
    (§2.2: "opaque boundary bytes").

    Mid-chunk starts (`start_offset > 0`): `parent_drh` is already the DRH
    value *after* folding the first touched chunk, so instead of folding
    forward the guest opens its preimage — `boundary_prev_drh` plus the
    recomputed hash of the first chunk must reproduce `parent_drh` — which
    binds the witnessed chunk to the chain position without requiring the
    guest to have derived `parent_drh` itself.

    Returns `(end_drh, end_offset)`.
    """
    chunk_count = len(chunks)
    if chunk_count == 0:
        raise Exception("rollup proof must touch at least one chunk")
    if not (0 <= start_offset < BLOB_BYTES_LENGTH):
        raise Exception("startOffset must be within [0, chunkSize)")

    end_offset = len(own_stream_bytes) - (chunk_count - 1) * BLOB_BYTES_LENGTH + start_offset
    if not (0 < end_offset <= BLOB_BYTES_LENGTH):
        raise Exception("chunk witness count is inconsistent with the reconstructed segment length")
    if len(opaque_prefix_bytes) != start_offset:
        raise Exception("opaquePrefixBytes length does not match startOffset")
    if len(opaque_suffix_bytes) != BLOB_BYTES_LENGTH - end_offset:
        raise Exception("opaqueSuffixBytes length does not match endOffset")

    setup = _trusted_setup()
    cursor = 0
    drh = parent_drh
    for i in range(chunk_count):
        is_first = i == 0
        is_last = i == chunk_count - 1
        prefix = opaque_prefix_bytes if is_first else b""
        suffix = opaque_suffix_bytes if is_last else b""
        own_slice_len = BLOB_BYTES_LENGTH - len(prefix) - len(suffix)

        own_slice = own_stream_bytes[cursor:cursor + own_slice_len]
        cursor += own_slice_len
        full_chunk_bytes = prefix + own_slice + suffix
        if len(full_chunk_bytes) != BLOB_BYTES_LENGTH:
            raise Exception(f"chunk {i} reconstructed bytes do not fill the chunk")

        try:
            # ┌─ PRECOMPILE (production guest): BLS12-381 / KZG commitment ───┐
            # │ The zkVM exposes EIP-4844 blob commitment computation as a    │
            # │ native primitive or a deterministic linked implementation.    │
            # │ This call hides the BLS12-381 multi-scalar multiplication     │
            # │ over the chunk's 4096 field elements.                         │
            # │                                                               │
            # │ Soundness for this chunk comes from computing the commitment  │
            # │ directly from `full_chunk_bytes` and matching its versioned   │
            # │ hash to the chunk's anchored hash: the commitment scheme's    │
            # │ binding property means only these exact bytes can produce a   │
            # │ commitment hashing to that value.                             │
            # └───────────────────────────────────────────────────────────────┘
            chunk_kzg_commitment = KZGCommitment(
                ckzg.blob_to_kzg_commitment(full_chunk_bytes, setup),
            )
        except Exception as exc:
            raise Exception("invalid chunk KZG commitment computation") from exc

        computed_chunk_hash = Hash32(kzg_commitment_to_versioned_hash(chunk_kzg_commitment))
        if computed_chunk_hash != chunks[i]:
            raise Exception(f"chunk {i} computed KZG commitment does not match chunkHash")

        if is_first and start_offset > 0:
            if boundary_prev_drh is None:
                raise Exception("mid-chunk start requires boundaryPrevDrh")
            if DrhWitness(boundary_prev_drh, chunks[i]).hash() != parent_drh:
                raise Exception("boundary chunk DRH preimage does not open parentDrh")
            drh = parent_drh
        else:
            drh = DrhWitness(drh, chunks[i]).hash()

    if cursor != len(own_stream_bytes):
        raise Exception("chunk witnesses do not cover the reconstructed segment length")

    return drh, end_offset


@dataclass
class RollupPublicInput:
    """
    The rollup / rollup-aggregation public input tuple from Readme.md section 2.4.

    `parent_block_hash` / `end_block_hash` are execution continuity — the role
    the old 3-input shnarf's `lastBlockHash` used to play — now explicit
    public-input fields rather than folded into the DA accumulator (§3.1).
    `start_offset` / `end_offset` are the byte positions (§3.4) that pair with
    `parent_drh` / `end_drh` to form this proof's start and end stream
    positions; `end_offset` is a derived output (computed from the guest's own
    recompression), not trusted witness input.

    `program_vks` is the set of ALL guest program VKs verified beneath this proof
    (§ProgramVK anchoring), checked against L1's single combined `approvedVks`
    set. It is semantically a SET, encoded as a CANONICAL sorted, distinct list
    (ascending by byte value): order carries no meaning, and sorting makes the
    commitment a pure function of the set's contents, so the guest and L1 agree
    by both canonicalizing rather than relying on incidental order. It is a plain
    public-input list field — no hash field — folded into L1's aggregate
    public-input hash like every other finalization field. L1 does not
    distinguish exec vs rollup VKs (a VK is a 32-byte commitment and the
    guarantee comes from recursive verification against `program_vk`, not from
    which output list it lands in), so the PI carries ONE list; the
    exec-vs-rollup distinction is internal guest bookkeeping only.
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
    parent_processed_ftx_number: U64
    end_ftx_rolling_hash: Hash32
    end_processed_ftx_number: U64
    filtered_addresses_hash: Hash32
    parent_drh: Hash32
    end_drh: Hash32
    parent_block_hash: Hash32
    end_block_hash: Hash32
    start_offset: int
    end_offset: int
    program_vks: List[Hash32] = field(default_factory=list)


@dataclass
class RollupProofPrivateInput:
    """
    Logical rollup request. One rollup proof covers >=1 consecutive whole
    conflations (each conflation = one l2-execution proof's block range,
    paired 1:1 with `l2_execution_proofs`), transported across >=1 chunks
    (§3.1).

    `parent_drh` / `start_offset` give this proof's start stream position
    (§3.4). `opaque_prefix_bytes` / `opaque_suffix_bytes` are foreign bytes
    not owned by this proof, relevant only at the two ends of the touched
    chunk range (not per-chunk) — see `_verify_and_fold_chunks`.
    `boundary_prev_drh` is required only for a mid-chunk start
    (`start_offset > 0`) — the DRH value before the first touched chunk, used
    to open its preimage. `end_drh` and `end_offset` are not request inputs:
    the guest derives them from its own recompression.

    `chain_id` is needed for sender recovery during DA truncation (§2.2
    step 2). It is committed transitively via the l2-execution proofs'
    `dynamicChainConfigHash` PI field — `assert_l2_execution_continuity`
    (step 8) ensures the same value flows across all l2-execution proofs in
    the rollup range, so the rollup proof inherits chain-config integrity
    from the l2-execution proofs it recursively verifies.
    """
    parent_drh: Hash32
    start_offset: int
    chain_id: U64
    conflations: List[ConflationWitness]
    chunks: List[Hash32]
    l2_execution_proofs: List[VerifiableL2ExecutionProof]
    opaque_prefix_bytes: bytes = b""
    opaque_suffix_bytes: bytes = b""
    boundary_prev_drh: Optional[Hash32] = None


@dataclass
class RollupProof:
    """
    A rollup proof as the rollup guest emits it: the guest *output* (the
    20-field `public_inputs` tuple + the root/address preimages) plus the
    `proof` bytes the aggregation guest recursively verifies.

    Guest/prover boundary: the guest emits `public_inputs` and the preimage
    lists only; `proof` is attached by the zkVM/prover layer above — a guest
    cannot prove itself — and is a placeholder (`b""`) in this reference.

    `end_block_number` is intentionally absent: it is already
    `public_inputs.end_block_number`. Only `start_block_number` (not in the PI
    tuple) is carried, so the aggregation guest can verify proof tiling.

    This type has no verifying-key field: a guest cannot attest its own VK, so
    `run_rollup_guest` never produces one. See `VerifiableRollupProof` for the
    coordinator-populated wrapper the rollup-aggregation guest actually
    consumes.
    """
    public_inputs: RollupPublicInput
    start_block_number: U64
    proof: bytes = b""
    l2_l1_roots: List[Hash32] = field(default_factory=list)
    filtered_addresses: List[Address] = field(default_factory=list)


@dataclass
class VerifiableRollupProof:
    """
    A `RollupProof` paired with the `program_vk` the rollup-aggregation guest
    recursively verifies it against (§ProgramVK anchoring).

    `program_vk` is a *runtime input* the coordinator supplies — the same
    value it verifies `proof` against here is the value merged into the
    aggregation guest's `program_vks` public output, so the anchored VK is
    provably the key the verification ran against. Never produced by
    `run_rollup_guest` (a guest cannot attest its own VK); only the
    rollup-aggregation request codec constructs this wrapper.
    """
    proof: RollupProof
    program_vk: Hash32


def run_rollup_guest(rollup_input: RollupProofPrivateInput) -> RollupProof:
    """
    rollup: for each conflation, independently computes the canonical
    compressed segment from `block_rlps` (truncate → RLP-encode →
    LZ4-compress, length-prefixed, §3.1) and concatenates the segments into
    this proof's own byte stream. Slices that stream across the chunks it
    touches, reconstructing each chunk's full published bytes together with
    any witnessed opaque boundary bytes, computes the KZG commitment for each,
    and checks it against the L1-anchored `chunkHash` — folding the DRH
    chain across the touched chunks as it goes (§3.4). Recursively verifies
    the N l2-execution proofs, checks continuity, builds the L2->L1
    Merkle-root commitment, collects FTX outputs, and emits the 20-field
    rollup PI tuple (§2.4).
    """
    if len(rollup_input.conflations) == 0:
        raise Exception("rollup proof must cover at least one conflation")
    if len(rollup_input.l2_execution_proofs) == 0:
        raise Exception("rollup proof must consume at least one l2-execution proof")
    if len(rollup_input.conflations) != len(rollup_input.l2_execution_proofs):
        raise Exception("conflations must pair 1:1 with l2-execution proofs")

    truncated_blocks: List[TruncatedEthereumBlock] = []
    parent_hashes: List[Hash32] = []
    segments: List[bytes] = []

    for conflation, verifiable_proof in zip(
        rollup_input.conflations, rollup_input.l2_execution_proofs,
    ):
        proof = verifiable_proof.proof
        expected_block_count = (
            int(proof.public_inputs.end_block_number) - int(proof.start_block_number) + 1
        )
        conflation_truncated, conflation_parent_hashes = _truncate_conflation(
            conflation.block_rlps, rollup_input.chain_id,
        )
        if len(conflation_truncated) != expected_block_count:
            raise Exception("conflation block count is inconsistent with its l2-execution proof range")
        if len(conflation_truncated) == 0:
            raise Exception("rollup proof cannot include an empty conflation")

        truncated_blocks.extend(conflation_truncated)
        parent_hashes.extend(conflation_parent_hashes)
        segments.append(_compress_conflation_segment(conflation_truncated))

    own_stream_bytes = b"".join(segments)
    end_drh, end_offset = _verify_and_fold_chunks(
        own_stream_bytes,
        rollup_input.start_offset,
        rollup_input.chunks,
        rollup_input.opaque_prefix_bytes,
        rollup_input.opaque_suffix_bytes,
        rollup_input.parent_drh,
        rollup_input.boundary_prev_drh,
    )

    rollup_start_block_number = int(rollup_input.l2_execution_proofs[0].proof.start_block_number)
    rollup_end_block_number = int(
        rollup_input.l2_execution_proofs[-1].proof.public_inputs.end_block_number
    )
    # Unwrap once: everything below except the recursive-verify loop only needs
    # the guest-emitted `L2ExecutionProof`, not the coordinator-attached VK.
    l2_execution_proofs = [vp.proof for vp in rollup_input.l2_execution_proofs]
    verify_l2_execution_proof_tiling(
        l2_execution_proofs,
        rollup_start_block_number,
        rollup_end_block_number,
    )

    concatenated_froms: List[Address] = []
    concatenated_l2_l1_messages: List[Hash32] = []
    concatenated_filtered_addresses: List[Address] = []
    truncated_froms: List[Address] = []
    truncated_block_hashes = [block.block_hash for block in truncated_blocks]

    for verifiable_proof in rollup_input.l2_execution_proofs:
        verify_l2_execution_proof(verifiable_proof.program_vk, verifiable_proof.proof)
        concatenated_froms.extend(verifiable_proof.proof.tx_froms)
        concatenated_l2_l1_messages.extend(verifiable_proof.proof.l2_l1_messages)
        concatenated_filtered_addresses.extend(verifiable_proof.proof.filtered_addresses)

    # The exec program VKs verified beneath this rollup proof, emitted as a
    # CANONICAL sorted, distinct list (§ProgramVK anchoring): semantically a set,
    # sorted so the commitment is a pure function of its contents. `Hash32` is a
    # bytes subclass, so `sorted` orders ascending by byte value.
    program_vks = sorted({vp.program_vk for vp in rollup_input.l2_execution_proofs})

    for block in truncated_blocks:
        truncated_froms.extend(block.froms)

    if concatenated_froms != truncated_froms:
        raise Exception("l2-execution proof txFroms do not match blob blockData.froms")

    first_proof = l2_execution_proofs[0]
    last_proof = l2_execution_proofs[-1]
    for proof in l2_execution_proofs:
        boundary_index = int(proof.public_inputs.end_block_number) - rollup_start_block_number
        if boundary_index < 0 or boundary_index >= len(truncated_block_hashes):
            raise Exception("l2-execution proof boundary falls outside the conflation block range")
        if proof.public_inputs.end_block_hash != truncated_block_hashes[boundary_index]:
            raise Exception("l2-execution proof end block hash does not match conflation data at its boundary")

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

    for left, right in zip(l2_execution_proofs, l2_execution_proofs[1:]):
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
        parent_processed_ftx_number=first_proof.public_inputs.parent_processed_ftx_number,
        end_ftx_rolling_hash=last_proof.public_inputs.end_ftx_rolling_hash,
        end_processed_ftx_number=last_proof.public_inputs.end_processed_ftx_number,
        filtered_addresses_hash=hash_address_list(concatenated_filtered_addresses),
        parent_drh=rollup_input.parent_drh,
        end_drh=end_drh,
        parent_block_hash=first_proof.public_inputs.parent_block_hash,
        end_block_hash=last_proof.public_inputs.end_block_hash,
        start_offset=rollup_input.start_offset,
        end_offset=end_offset,
        program_vks=program_vks,
    )

    return RollupProof(
        public_inputs=public_inputs,
        start_block_number=U64(rollup_start_block_number),
        l2_l1_roots=l2_l1_roots,
        filtered_addresses=concatenated_filtered_addresses,
    )


def recursive_stark_verify(program_vk: Hash32, proof: bytes) -> None:
    """PRECOMPILE (production guest): the zkVM in-circuit recursive STARK
    verifier. Accepts iff `proof` verifies under `program_vk`. Stubbed in this
    reference; the point is that the SAME `program_vk` passed here is the value
    the caller emits into `program_vks`, so the anchored VK is provably the
    verification key (no divergence between what is verified and what L1 checks)."""
    return None


def verify_l2_execution_proof(program_vk: Hash32, proof: L2ExecutionProof) -> None:
    """
    Verify an inner l2-execution proof against its claimed public inputs.

    Recursive STARK verification is a zkVM primitive in the guest; `program_vk`
    is the explicit verify-key parameter it checks against (§ProgramVK
    anchoring). The rollup guest passes the same `program_vk` it bubbles up into
    `exec_vks` / `program_vks`, so the anchored VK is provably the key the
    verification ran against. `L2ExecutionProof.proof` stands in for those
    recursive-STARK bytes; beyond the recursive verify, the reference only
    re-checks the hash-preimage bindings (`txFromsHash`, `l2L1MessagesHash`,
    `filteredAddressesHash`) the rollup proof consumes alongside the PI tuple.
    """
    # First: the recursive STARK verify against the explicit verify key.
    recursive_stark_verify(program_vk, proof.proof)
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
        expected_start = proof.public_inputs.end_block_number + 1
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
    if left.end_processed_ftx_number != right.parent_processed_ftx_number:
        raise Exception("l2-execution processed-FTX-number continuity failed")


def build_l2_messages_tree(msgs: Sequence[Hash32]) -> Tuple[List[Hash32], Hash32]:
    """
    Build L2-to-L1 message trees exactly as specified:
    - Pad the ordered message-hash list with zero Hash32 values until its
      length is a multiple of 32.
    - Split the padded list into consecutive 32-leaf chunks.
    - Merkle-hash each chunk as a complete depth-5 binary tree with keccak.
    - Flat-hash the ordered roots with keccak256(root_1 || ... || root_n).

    The returned root list is the private preimage used by aggregation and L1
    calldata; the returned hash is the public `l2L1BridgeTransactionTree`.
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
    hands the padded result to `ckzg.blob_to_kzg_commitment` (§2.2 step 1).

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
