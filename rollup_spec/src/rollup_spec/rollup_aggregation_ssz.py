"""
Canonical SSZ wire format for the rollup-aggregation guest program.

This module is the contract a Zig guest decoder and a future Go encoder are
written against: remerkleable SSZ containers that mirror the logical
dataclasses in `rollup_aggregation.py` / `l1_rollup.py` / `rollup.py`, plus
the framing and bounds needed to serialize them unambiguously. This Python
spec is the source of truth for the wire format. The framing helpers are imported from
the sibling `l2_execution_ssz.py`, and the shared `SszRollupPublicInput` container plus common bounds
are imported from the sibling `rollup_ssz.py`. Finalization has its own
public-input container with the proven messaging offsets and no block count.

Inputs are framed as `schema_id (2 bytes, big-endian) || SSZ bytes`.
Outputs use `keccak256(schema_id || SSZ(public_inputs)) || schema_id ||
SSZ(public_inputs)`. Two schema ids are defined, one per guest-facing message:

  - `ROLLUP_AGGREGATION_INPUT_SCHEMA_ID`  (0x1002) — rollup-aggregation guest input
  - `ROLLUP_AGGREGATION_OUTPUT_SCHEMA_ID` (0x1802) — rollup-aggregation guest output

The guest output container omits the `proof` field the logical
`FinalizationSubmission` dataclass carries: a guest cannot attest its own
proof, so `proof` is attached by the prover layer above and is never part of
the guest-emitted bytes. Decoding an output frame reconstructs the dataclass
with `proof=b""`, matching the placeholder `run_rollup_aggregation_guest`
already emits.

List/vector bounds are conservative powers of two: capacity ceilings for the
wire format, not the guest's or coordinator's own (tighter) operational
limits. Each constant below carries a one-line rationale.
"""

from typing import Any

from ethereum.crypto.hash import Hash32, keccak256
from ethereum.state import Address
from ethereum_types.numeric import U64
from remerkleable.basic import uint16, uint64
from remerkleable.byte_arrays import ByteList, Bytes32 as SszBytes32
from remerkleable.complex import Container, List

from .l1_rollup import FinalizationPublicInput, FinalizationSubmission
from .stateless_input import InvalidSsz
from .rollup import RollupProof, VerifiableRollupProof
from .rollup_aggregation import RollupAggregationProofPrivateInput
from .l2_execution_ssz import (
    MAX_PROOF_BYTES,
    SszAddress,
    _frame,
    _strict_decode,
    _strip_frame,
)
from .rollup_ssz import (
    SszRollupPublicInput,
    MAX_L2_L1_ROOTS,
    MAX_FILTERED_ADDRESSES,
    MAX_PROGRAM_VKS,
    _rollup_public_input_from_view,
    _ssz_rollup_public_input,
)

# ── Framing ──────────────────────────────────────────────────────────────────
ROLLUP_AGGREGATION_INPUT_SCHEMA_ID = 0x1002
ROLLUP_AGGREGATION_OUTPUT_SCHEMA_ID = 0x1802

# ── SSZ list/vector bounds ───────────────────────────────────────────────────
MAX_L2_MESSAGING_BLOCKS_OFFSETS = 2**16        # L1 calldata offsets carried by the aggregation output
MAX_ROLLUP_PROOFS_PER_AGGREGATION = 2**10      # rollup proofs recursively verified by one aggregation proof

# ── SSZ wire schema (remerkleable) ───────────────────────────────────────────


class SszRollupProof(Container):
    # An already-proven rollup proof, as recursively verified by the
    # rollup-aggregation guest: carries its own `proof` bytes.
    public_inputs: SszRollupPublicInput
    start_block_number: uint64
    proof: ByteList[MAX_PROOF_BYTES]


class SszVerifiableRollupProof(Container):
    proof: SszRollupProof
    program_vk: SszBytes32


class SszRollupAggregationProofPrivateInput(Container):
    # Field order matches `rollup_aggregation.py::RollupAggregationProofPrivateInput`.
    rollup_proofs: List[SszVerifiableRollupProof, MAX_ROLLUP_PROOFS_PER_AGGREGATION]


class SszFinalizationPublicInput(Container):
    end_block_number: uint64
    end_block_timestamp: uint64
    parent_l1_l2_bridge_rolling_hash: SszBytes32
    parent_l1_l2_bridge_rolling_hash_message_number: uint64
    end_l1_l2_bridge_rolling_hash: SszBytes32
    end_l1_l2_bridge_rolling_hash_message_number: uint64
    dynamic_chain_config_hash: SszBytes32
    parent_ftx_rolling_hash: SszBytes32
    parent_ftx_number: uint64
    end_ftx_rolling_hash: SszBytes32
    end_processed_ftx_number: uint64
    parent_data_rolling_hash: SszBytes32
    end_data_rolling_hash: SszBytes32
    parent_block_hash: SszBytes32
    end_block_hash: SszBytes32
    start_offset: uint64
    end_offset: uint64
    l2_l1_tree_depth: uint64
    l2_l1_roots: List[SszBytes32, MAX_L2_L1_ROOTS]
    filtered_addresses: List[SszAddress, MAX_FILTERED_ADDRESSES]
    program_ids: List[SszBytes32, MAX_PROGRAM_VKS]
    l2_messaging_blocks_offsets: List[uint16, MAX_L2_MESSAGING_BLOCKS_OFFSETS]


# ── Logical dataclass -> SSZ view converters ─────────────────────────────────


def _ssz_rollup_proof(proof: RollupProof) -> SszRollupProof:
    return SszRollupProof(
        public_inputs=_ssz_rollup_public_input(proof.public_inputs),
        start_block_number=int(proof.start_block_number),
        proof=bytes(proof.proof),
    )


def _ssz_verifiable_rollup_proof(verifiable: VerifiableRollupProof) -> SszVerifiableRollupProof:
    return SszVerifiableRollupProof(
        proof=_ssz_rollup_proof(verifiable.proof),
        program_vk=bytes(verifiable.program_vk),
    )


# ── SSZ view -> logical dataclass converters ─────────────────────────────────


def _rollup_proof_from_view(view: Any) -> RollupProof:
    return RollupProof(
        public_inputs=_rollup_public_input_from_view(view.public_inputs),
        start_block_number=U64(int(view.start_block_number)),
        proof=bytes(view.proof),
    )


def _verifiable_rollup_proof_from_view(view: Any) -> VerifiableRollupProof:
    return VerifiableRollupProof(
        proof=_rollup_proof_from_view(view.proof),
        program_vk=Hash32(bytes(view.program_vk)),
    )


# ── Rollup-aggregation input: dataclass <-> framed SSZ bytes ─────────────────


def encode_aggregation_input(agg_input: RollupAggregationProofPrivateInput) -> bytes:
    """Encode a `RollupAggregationProofPrivateInput` into framed SSZ bytes (0x1002 schema id)."""
    ssz_input = SszRollupAggregationProofPrivateInput(
        rollup_proofs=[_ssz_verifiable_rollup_proof(p) for p in agg_input.rollup_proofs],
    )
    return _frame(ROLLUP_AGGREGATION_INPUT_SCHEMA_ID, ssz_input.encode_bytes())


def decode_aggregation_input_ssz(data: bytes) -> RollupAggregationProofPrivateInput:
    """
    Decode framed SSZ bytes into a `RollupAggregationProofPrivateInput`. Strict:
    rejects a wrong schema id, truncated bytes, trailing bytes, or non-canonical SSZ.
    """
    payload = _strip_frame(data, ROLLUP_AGGREGATION_INPUT_SCHEMA_ID, "rollup-aggregation input")
    view = _strict_decode(payload, SszRollupAggregationProofPrivateInput)
    return RollupAggregationProofPrivateInput(
        rollup_proofs=[_verifiable_rollup_proof_from_view(p) for p in view.rollup_proofs],
    )


# ── Rollup-aggregation output: dataclass <-> framed SSZ bytes ────────────────


def encode_aggregation_output(submission: FinalizationSubmission) -> bytes:
    """
    Encode the rollup-aggregation guest's own output into framed SSZ bytes
    (0x1802 schema id). `submission.proof` is deliberately dropped — it is a
    prover-attached placeholder in `FinalizationSubmission`, never part of the
    guest-emitted bytes.
    """
    pi = submission.public_inputs
    fields = {name: getattr(pi, name) for name in SszFinalizationPublicInput.fields()}
    for name in (
        "end_block_number", "end_block_timestamp", "parent_l1_l2_bridge_rolling_hash_message_number",
        "end_l1_l2_bridge_rolling_hash_message_number", "parent_ftx_number", "end_processed_ftx_number",
    ):
        fields[name] = int(fields[name])
    for name in (
        "parent_l1_l2_bridge_rolling_hash", "end_l1_l2_bridge_rolling_hash", "dynamic_chain_config_hash",
        "parent_ftx_rolling_hash", "end_ftx_rolling_hash", "parent_data_rolling_hash", "end_data_rolling_hash",
        "parent_block_hash", "end_block_hash",
    ):
        fields[name] = bytes(fields[name])
    fields["l2_l1_roots"] = [bytes(root) for root in pi.l2_l1_roots]
    fields["filtered_addresses"] = [bytes(address) for address in pi.filtered_addresses]
    fields["program_ids"] = [bytes(program_id) for program_id in pi.program_ids]
    preimage = _frame(ROLLUP_AGGREGATION_OUTPUT_SCHEMA_ID, SszFinalizationPublicInput(**fields).encode_bytes())
    return keccak256(preimage) + preimage


def decode_aggregation_output_ssz(data: bytes) -> FinalizationSubmission:
    """
    Decode framed SSZ bytes into a `FinalizationSubmission` with `proof=b""`
    (the guest never emits proof bytes; the prover layer attaches them
    separately). Strict: rejects a wrong schema id, truncated bytes, trailing
    bytes, or non-canonical SSZ.
    """
    if len(data) < 34 or keccak256(data[32:]) != data[:32]:
        raise InvalidSsz("rollup-aggregation output: invalid public-input commitment")
    payload = _strip_frame(data[32:], ROLLUP_AGGREGATION_OUTPUT_SCHEMA_ID, "rollup-aggregation output")
    view = _strict_decode(payload, SszFinalizationPublicInput)
    fields = {name: getattr(view, name) for name in SszFinalizationPublicInput.fields()}
    for name in (
        "end_block_number", "end_block_timestamp", "parent_l1_l2_bridge_rolling_hash_message_number",
        "end_l1_l2_bridge_rolling_hash_message_number", "parent_ftx_number", "end_processed_ftx_number",
    ):
        fields[name] = U64(int(fields[name]))
    for name in (
        "parent_l1_l2_bridge_rolling_hash", "end_l1_l2_bridge_rolling_hash", "dynamic_chain_config_hash",
        "parent_ftx_rolling_hash", "end_ftx_rolling_hash", "parent_data_rolling_hash", "end_data_rolling_hash",
        "parent_block_hash", "end_block_hash",
    ):
        fields[name] = Hash32(bytes(fields[name]))
    for name in ("start_offset", "end_offset", "l2_l1_tree_depth"):
        fields[name] = int(fields[name])
    fields["l2_l1_roots"] = [Hash32(bytes(root)) for root in view.l2_l1_roots]
    fields["filtered_addresses"] = [Address(bytes(address)) for address in view.filtered_addresses]
    fields["program_ids"] = [Hash32(bytes(program_id)) for program_id in view.program_ids]
    fields["l2_messaging_blocks_offsets"] = [int(offset) for offset in view.l2_messaging_blocks_offsets]
    return FinalizationSubmission(
        public_inputs=FinalizationPublicInput(**fields),
        proof=b"",
    )
