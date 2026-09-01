"""
Canonical SSZ wire format for the rollup guest program.

Remerkleable SSZ containers that mirror the logical dataclasses in
`rollup.py`, plus the framing and bounds needed to serialize them
unambiguously. The framing machinery and the embedded l2-execution proof
containers (`SszVerifiableL2ExecutionProof` and its parts) come from the
sibling `l2_execution_ssz.py`, mirroring the proof pipeline itself.

Framing: every message is `schema_id (2 bytes, big-endian) || SSZ bytes`.
Two schema ids are defined, one per guest-facing message:

  - `ROLLUP_INPUT_SCHEMA_ID`  (0x1001) — rollup guest input
  - `ROLLUP_OUTPUT_SCHEMA_ID` (0x1801) — rollup guest output

The guest output container omits the `proof` field the logical `RollupProof`
dataclass carries: a guest cannot attest its own proof, so `proof` is attached
by the prover layer above and is never part of the guest-emitted bytes.
Decoding an output frame reconstructs the dataclass with `proof=b""`, matching
the placeholder `run_rollup_guest` already emits.

Optional modelling: `RollupProofPrivateInput.boundary_prev_data_rolling_hash`
is the one Optional field in this wire format (required only for a mid-chunk
start, `start_offset > 0`). It is modelled as `List[Bytes32, 1]` — an
SSZ list capped at length 1 — exactly as `stateless_input.py` models its
optional fork-activation values. Absent is the empty list; present is a
single-element list. This keeps every field a plain SSZ list/container type
with no bespoke union/optional primitive.

List/vector bounds are conservative powers of two: capacity ceilings for the
wire format, not the guest's or coordinator's own (tighter) operational
limits. Each constant below carries a one-line rationale.
"""

from typing import Any, Optional

from ethereum.crypto.hash import Hash32
from ethereum.state import Address
from ethereum_types.numeric import U64
from remerkleable.basic import uint64
from remerkleable.byte_arrays import ByteList, Bytes32 as SszBytes32
from remerkleable.complex import Container, List

from .l2_execution_ssz import (
    SszAddress,
    SszVerifiableL2ExecutionProof,
    _frame,
    _ssz_verifiable_l2_execution_proof,
    _strict_decode,
    _strip_frame,
    _verifiable_l2_execution_proof_from_view,
)
from .rollup import (
    BLOB_BYTES_LENGTH,
    ConflationWitness,
    RollupProof,
    RollupProofPrivateInput,
    RollupPublicInput,
)

# ── Framing ──────────────────────────────────────────────────────────────────
ROLLUP_INPUT_SCHEMA_ID = 0x1001
ROLLUP_OUTPUT_SCHEMA_ID = 0x1801

# ── SSZ list/vector bounds ───────────────────────────────────────────────────
MAX_CONFLATIONS_PER_ROLLUP = 2**10             # conflations one rollup proof recursively verifies
MAX_L2_EXECUTION_PROOFS_PER_ROLLUP = 2**10     # paired 1:1 with conflations (rollup.py::run_rollup_guest)
MAX_CHUNKS_PER_ROLLUP = 2**12                  # chunks touched by one rollup proof's dataRollingHash fold
MAX_BLOCK_RLPS_PER_CONFLATION = 2**12          # full block RLPs (one per block) in a single conflation
MAX_BYTES_PER_BLOCK_RLP = 2**24                # 16 MiB: a full canonical block RLP including all tx bodies
MAX_PROGRAM_VKS = 2**10                        # distinct guest program VKs bubbled into one program_vks set
MAX_L2_L1_ROOTS = 2**16                        # per-chunk L2->L1 message-tree roots merged into one proof
MAX_FILTERED_ADDRESSES = 2**16                 # sanction-list addresses merged at the rollup layer
# `opaque_prefix_bytes`/`opaque_suffix_bytes` are each bounded by one chunk
# (`_verify_and_fold_chunks` in rollup.py). Their exact lengths are constrained
# by `start_offset`/`end_offset`, so `BLOB_BYTES_LENGTH` is a safe SSZ bound.

# ── SSZ wire schema (remerkleable) ───────────────────────────────────────────


class SszConflationWitness(Container):
    block_rlps: List[ByteList[MAX_BYTES_PER_BLOCK_RLP], MAX_BLOCK_RLPS_PER_CONFLATION]


class SszRollupPublicInput(Container):
    # 20-field rollup public input tuple (Readme.md §2.4), field order matches
    # `rollup.py::RollupPublicInput`.
    end_block_number: uint64
    end_block_timestamp: uint64
    l2_l1_bridge_transaction_tree: SszBytes32
    parent_l1_l2_bridge_rolling_hash: SszBytes32
    parent_l1_l2_bridge_rolling_hash_message_number: uint64
    end_l1_l2_bridge_rolling_hash: SszBytes32
    end_l1_l2_bridge_rolling_hash_message_number: uint64
    dynamic_chain_config_hash: SszBytes32
    parent_ftx_rolling_hash: SszBytes32
    parent_ftx_number: uint64
    end_ftx_rolling_hash: SszBytes32
    end_processed_ftx_number: uint64
    filtered_addresses_hash: SszBytes32
    parent_data_rolling_hash: SszBytes32
    end_data_rolling_hash: SszBytes32
    parent_block_hash: SszBytes32
    end_block_hash: SszBytes32
    start_offset: uint64
    end_offset: uint64
    program_vks: List[SszBytes32, MAX_PROGRAM_VKS]


class SszRollupProofPrivateInput(Container):
    # Field order matches `rollup.py::RollupProofPrivateInput`.
    parent_data_rolling_hash: SszBytes32
    start_offset: uint64
    chain_id: uint64
    conflations: List[SszConflationWitness, MAX_CONFLATIONS_PER_ROLLUP]
    chunks: List[SszBytes32, MAX_CHUNKS_PER_ROLLUP]
    l2_execution_proofs: List[SszVerifiableL2ExecutionProof, MAX_L2_EXECUTION_PROOFS_PER_ROLLUP]
    opaque_prefix_bytes: ByteList[BLOB_BYTES_LENGTH]
    opaque_suffix_bytes: ByteList[BLOB_BYTES_LENGTH]
    # Optional[Hash32]: empty list means absent, single-element list means
    # present (see module docstring).
    boundary_prev_data_rolling_hash: List[SszBytes32, 1]


class SszRollupOutput(Container):
    # The rollup guest's own output: `RollupProof` with `proof` omitted (the
    # prover layer attaches it). Field order matches the remaining fields of
    # `rollup.py::RollupProof`.
    public_inputs: SszRollupPublicInput
    start_block_number: uint64
    l2_l1_roots: List[SszBytes32, MAX_L2_L1_ROOTS]
    filtered_addresses: List[SszAddress, MAX_FILTERED_ADDRESSES]


# ── Logical dataclass -> SSZ view converters ─────────────────────────────────


def _ssz_conflation_witness(witness: ConflationWitness) -> SszConflationWitness:
    return SszConflationWitness(block_rlps=[bytes(r) for r in witness.block_rlps])


def _ssz_rollup_public_input(pi: RollupPublicInput) -> SszRollupPublicInput:
    return SszRollupPublicInput(
        end_block_number=int(pi.end_block_number),
        end_block_timestamp=int(pi.end_block_timestamp),
        l2_l1_bridge_transaction_tree=bytes(pi.l2_l1_bridge_transaction_tree),
        parent_l1_l2_bridge_rolling_hash=bytes(pi.parent_l1_l2_bridge_rolling_hash),
        parent_l1_l2_bridge_rolling_hash_message_number=int(
            pi.parent_l1_l2_bridge_rolling_hash_message_number
        ),
        end_l1_l2_bridge_rolling_hash=bytes(pi.end_l1_l2_bridge_rolling_hash),
        end_l1_l2_bridge_rolling_hash_message_number=int(
            pi.end_l1_l2_bridge_rolling_hash_message_number
        ),
        dynamic_chain_config_hash=bytes(pi.dynamic_chain_config_hash),
        parent_ftx_rolling_hash=bytes(pi.parent_ftx_rolling_hash),
        parent_ftx_number=int(pi.parent_ftx_number),
        end_ftx_rolling_hash=bytes(pi.end_ftx_rolling_hash),
        end_processed_ftx_number=int(pi.end_processed_ftx_number),
        filtered_addresses_hash=bytes(pi.filtered_addresses_hash),
        parent_data_rolling_hash=bytes(pi.parent_data_rolling_hash),
        end_data_rolling_hash=bytes(pi.end_data_rolling_hash),
        parent_block_hash=bytes(pi.parent_block_hash),
        end_block_hash=bytes(pi.end_block_hash),
        start_offset=int(pi.start_offset),
        end_offset=int(pi.end_offset),
        program_vks=[bytes(v) for v in pi.program_vks],
    )


def _ssz_rollup_input(private_input: RollupProofPrivateInput) -> SszRollupProofPrivateInput:
    boundary = private_input.boundary_prev_data_rolling_hash
    return SszRollupProofPrivateInput(
        parent_data_rolling_hash=bytes(private_input.parent_data_rolling_hash),
        start_offset=int(private_input.start_offset),
        chain_id=int(private_input.chain_id),
        conflations=[_ssz_conflation_witness(c) for c in private_input.conflations],
        chunks=[bytes(c) for c in private_input.chunks],
        l2_execution_proofs=[
            _ssz_verifiable_l2_execution_proof(p) for p in private_input.l2_execution_proofs
        ],
        opaque_prefix_bytes=bytes(private_input.opaque_prefix_bytes),
        opaque_suffix_bytes=bytes(private_input.opaque_suffix_bytes),
        boundary_prev_data_rolling_hash=[bytes(boundary)] if boundary is not None else [],
    )


# ── SSZ view -> logical dataclass converters ─────────────────────────────────


def _conflation_witness_from_view(view: Any) -> ConflationWitness:
    return ConflationWitness(block_rlps=[bytes(r) for r in view.block_rlps])


def _rollup_public_input_from_view(view: Any) -> RollupPublicInput:
    return RollupPublicInput(
        end_block_number=U64(int(view.end_block_number)),
        end_block_timestamp=U64(int(view.end_block_timestamp)),
        l2_l1_bridge_transaction_tree=Hash32(bytes(view.l2_l1_bridge_transaction_tree)),
        parent_l1_l2_bridge_rolling_hash=Hash32(bytes(view.parent_l1_l2_bridge_rolling_hash)),
        parent_l1_l2_bridge_rolling_hash_message_number=U64(
            int(view.parent_l1_l2_bridge_rolling_hash_message_number)
        ),
        end_l1_l2_bridge_rolling_hash=Hash32(bytes(view.end_l1_l2_bridge_rolling_hash)),
        end_l1_l2_bridge_rolling_hash_message_number=U64(
            int(view.end_l1_l2_bridge_rolling_hash_message_number)
        ),
        dynamic_chain_config_hash=Hash32(bytes(view.dynamic_chain_config_hash)),
        parent_ftx_rolling_hash=Hash32(bytes(view.parent_ftx_rolling_hash)),
        parent_ftx_number=U64(int(view.parent_ftx_number)),
        end_ftx_rolling_hash=Hash32(bytes(view.end_ftx_rolling_hash)),
        end_processed_ftx_number=U64(int(view.end_processed_ftx_number)),
        filtered_addresses_hash=Hash32(bytes(view.filtered_addresses_hash)),
        parent_data_rolling_hash=Hash32(bytes(view.parent_data_rolling_hash)),
        end_data_rolling_hash=Hash32(bytes(view.end_data_rolling_hash)),
        parent_block_hash=Hash32(bytes(view.parent_block_hash)),
        end_block_hash=Hash32(bytes(view.end_block_hash)),
        start_offset=int(view.start_offset),
        end_offset=int(view.end_offset),
        program_vks=[Hash32(bytes(v)) for v in view.program_vks],
    )


def _rollup_input_from_view(view: Any) -> RollupProofPrivateInput:
    boundary_list = view.boundary_prev_data_rolling_hash
    boundary: Optional[Hash32] = (
        Hash32(bytes(boundary_list[0])) if len(boundary_list) == 1 else None
    )
    return RollupProofPrivateInput(
        parent_data_rolling_hash=Hash32(bytes(view.parent_data_rolling_hash)),
        start_offset=int(view.start_offset),
        chain_id=U64(int(view.chain_id)),
        conflations=[_conflation_witness_from_view(c) for c in view.conflations],
        chunks=[Hash32(bytes(c)) for c in view.chunks],
        l2_execution_proofs=[
            _verifiable_l2_execution_proof_from_view(p) for p in view.l2_execution_proofs
        ],
        opaque_prefix_bytes=bytes(view.opaque_prefix_bytes),
        opaque_suffix_bytes=bytes(view.opaque_suffix_bytes),
        boundary_prev_data_rolling_hash=boundary,
    )


# ── Rollup input: dataclass <-> framed SSZ bytes ─────────────────────────────


def encode_rollup_input(private_input: RollupProofPrivateInput) -> bytes:
    """Encode a `RollupProofPrivateInput` into framed SSZ bytes (0x1001 schema id)."""
    return _frame(ROLLUP_INPUT_SCHEMA_ID, _ssz_rollup_input(private_input).encode_bytes())


def decode_rollup_input_ssz(data: bytes) -> RollupProofPrivateInput:
    """
    Decode framed SSZ bytes into a `RollupProofPrivateInput`. Strict: rejects a
    wrong schema id, truncated bytes, trailing bytes, or non-canonical SSZ.
    """
    payload = _strip_frame(data, ROLLUP_INPUT_SCHEMA_ID, "rollup input")
    return _rollup_input_from_view(_strict_decode(payload, SszRollupProofPrivateInput))


# ── Rollup output: dataclass <-> framed SSZ bytes ────────────────────────────


def encode_rollup_output(proof: RollupProof) -> bytes:
    """
    Encode the rollup guest's own output into framed SSZ bytes (0x1801 schema
    id). `proof.proof` is deliberately dropped — it is a prover-attached
    placeholder in `RollupProof`, never part of the guest-emitted bytes.
    """
    ssz_output = SszRollupOutput(
        public_inputs=_ssz_rollup_public_input(proof.public_inputs),
        start_block_number=int(proof.start_block_number),
        l2_l1_roots=[bytes(r) for r in proof.l2_l1_roots],
        filtered_addresses=[bytes(a) for a in proof.filtered_addresses],
    )
    return _frame(ROLLUP_OUTPUT_SCHEMA_ID, ssz_output.encode_bytes())


def decode_rollup_output_ssz(data: bytes) -> RollupProof:
    """
    Decode framed SSZ bytes into a `RollupProof` with `proof=b""` (the guest
    never emits proof bytes; the prover layer attaches them separately).
    Strict: rejects a wrong schema id, truncated bytes, trailing bytes, or
    non-canonical SSZ.
    """
    payload = _strip_frame(data, ROLLUP_OUTPUT_SCHEMA_ID, "rollup output")
    view = _strict_decode(payload, SszRollupOutput)
    return RollupProof(
        public_inputs=_rollup_public_input_from_view(view.public_inputs),
        start_block_number=U64(int(view.start_block_number)),
        l2_l1_roots=[Hash32(bytes(r)) for r in view.l2_l1_roots],
        filtered_addresses=[Address(bytes(a)) for a in view.filtered_addresses],
    )
