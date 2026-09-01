"""
Canonical SSZ wire format for the l2-execution guest program.

Remerkleable SSZ containers that mirror the logical dataclasses in
`l2_execution.py` / `block.py`, plus the framing and bounds needed to
serialize them unambiguously. As the base guest of the proof pipeline it
also hosts the framing machinery the sibling guest wire modules import. The
vanilla per-block payload wire (schema id 0x0001) lives in
`stateless_input.py`.

Framing: every message is `schema_id (2 bytes, big-endian) || SSZ bytes`.
Two schema ids are defined here:

  - `L2_EXECUTION_INPUT_SCHEMA_ID`  (0x0002) — extended l2-execution guest input
  - `L2_EXECUTION_OUTPUT_SCHEMA_ID` (0x0003) — extended l2-execution guest output

The guest output wire is hash-only: the 0x0003 body is exactly
`keccak256(ssz(public_inputs))` — 32 bytes, nothing else (34 bytes framed).
The remaining `L2ExecutionProof` fields (`start_block_number` and the
`l2_l1_messages`/`tx_froms`/`filtered_addresses` preimages) are off-chain
data, never part of this wire format, and `proof` is attached by the prover
layer above the guest. The hash is irreversible, so the output decoder
returns the public-inputs hash rather than reconstructing a dataclass;
`encode_l2_execution_public_inputs_bytes` exposes the fixed 368-byte
preimage tuple.

Each payload's `stateless_input_ssz` is carried opaquely — an already
0x0001-framed vanilla stateless-input byte slice, byte-identical to what a
vanilla guest accepts, decoded on its own one level up — so this codec never
looks inside it.

Besides the input wire, this module carries the SSZ containers for an
already-proven l2-execution proof (`SszVerifiableL2ExecutionProof` and its
parts): they are not a guest message of their own but are embedded in the
rollup guest's input, which recursively verifies them.

List/vector bounds are conservative powers of two: capacity ceilings for the
wire format, not the guest's or coordinator's own (tighter) operational
limits. Each constant below carries a one-line rationale.
"""

from typing import Any, TypeAlias

from ethereum.crypto.hash import Hash32, keccak256
from ethereum.state import Address
from ethereum_types.numeric import U64
from remerkleable.basic import uint8, uint64
from remerkleable.byte_arrays import ByteList, ByteVector, Bytes32 as SszBytes32
from remerkleable.complex import Container, List

from .block import (
    ChainConfig,
    ForcedTransactionAcceptance,
    ForcedTransactionWitness,
    LinethPayloadInput,
    LinethRollupExtension,
)
from .l2_execution import (
    L2ExecutionProof,
    L2ExecutionProofPrivateInput,
    L2ExecutionProofPublicInput,
    VerifiableL2ExecutionProof,
)
from .stateless_input import InvalidSsz

# ── Framing ──────────────────────────────────────────────────────────────────
L2_EXECUTION_INPUT_SCHEMA_ID = 0x0002
L2_EXECUTION_OUTPUT_SCHEMA_ID = 0x0003
SCHEMA_ID_SIZE = 2

# ── SSZ list/vector bounds ───────────────────────────────────────────────────
MAX_PAYLOADS = 2**16                           # blocks (one payload each) in one l2-execution proof range
MAX_FTX_PER_PAYLOAD = 2**16                    # forced transactions declared for a single payload
MAX_STATELESS_INPUT_BYTES = 2**30              # 1 GiB: one opaque, already-framed vanilla stateless input
MAX_TX_BYTES = 2**30                           # matches the consensus-layer Transaction ByteList limit
MAX_PROOF_BYTES = 2**24                        # 16 MiB: generous ceiling on a recursively-verified proof blob
MAX_L2_L1_MESSAGES_PER_EXEC_PROOF = 2**16      # L2->L1 message hashes emitted by one l2-execution proof
MAX_TX_FROMS_PER_EXEC_PROOF = 2**16            # recovered tx senders emitted by one l2-execution proof
MAX_FILTERED_ADDRESSES_PER_EXEC_PROOF = 2**16  # sanction-list addresses emitted by one l2-execution proof

SszAddress: TypeAlias = ByteVector[20]

# ── Framing helpers ──────────────────────────────────────────────────────────


def _frame(schema_id: int, raw: bytes) -> bytes:
    return schema_id.to_bytes(SCHEMA_ID_SIZE, "big") + raw


def _strip_frame(data: bytes, schema_id: int, ctx: str) -> bytes:
    payload = bytes(data)
    if len(payload) < SCHEMA_ID_SIZE:
        raise InvalidSsz(f"{ctx}: input shorter than the {SCHEMA_ID_SIZE}-byte schema id")
    got = int.from_bytes(payload[:SCHEMA_ID_SIZE], "big")
    if got != schema_id:
        raise InvalidSsz(f"{ctx}: expected schema id 0x{schema_id:04x}, got 0x{got:04x}")
    return payload[SCHEMA_ID_SIZE:]


def _strict_decode(data: bytes, container: type) -> Any:
    """
    Decode `data` as `container`, rejecting anything that is not its canonical
    SSZ encoding. `remerkleable.decode_bytes` is lax about length (it ignores or
    absorbs trailing bytes), so we re-encode and require equality — SSZ encoding
    is bijective.
    """
    try:
        view = container.decode_bytes(data)
    except Exception as exc:  # remerkleable raises a variety of decode errors
        raise InvalidSsz(f"{container.__name__}: {exc}") from exc
    if view.encode_bytes() != data:
        raise InvalidSsz(f"{container.__name__}: input is not the canonical SSZ encoding")
    return view


# ── SSZ wire schema: extended guest input ────────────────────────────────────


class SszChainConfig(Container):
    # Field order matches `block.py::ChainConfig`.
    l2_message_service_address: SszAddress
    coinbase: SszAddress
    chain_id: uint64


class SszForcedTransactionWitness(Container):
    # Field order matches `block.py::ForcedTransactionWitness`; `acceptance`
    # carries the `ForcedTransactionAcceptance` enum value (0..4).
    number: uint64
    signed_tx_rlp: ByteList[MAX_TX_BYTES]
    acceptance: uint8
    deadline: uint64


class SszLinethPayloadInput(Container):
    # `stateless_input_ssz` is the opaque, already 0x0001-framed vanilla
    # stateless-input byte slice.
    stateless_input_ssz: ByteList[MAX_STATELESS_INPUT_BYTES]
    forced_transactions: List[SszForcedTransactionWitness, MAX_FTX_PER_PAYLOAD]


class SszL2ExecutionProofPrivateInput(Container):
    # Wire order: `chain_config` before `payloads` (see module docstring).
    parent_ftx_rolling_hash: SszBytes32
    parent_last_processed_ftx_number: uint64
    chain_config: SszChainConfig
    payloads: List[SszLinethPayloadInput, MAX_PAYLOADS]


# ── SSZ wire schema: embedded proof containers ───────────────────────────────


class SszL2ExecutionProofPublicInput(Container):
    # 16-field l2-execution public input tuple (Readme.md §2.1), field order
    # matches `l2_execution.py::L2ExecutionProofPublicInput`.
    parent_block_hash: SszBytes32
    end_block_hash: SszBytes32
    end_block_number: uint64
    end_block_timestamp: uint64
    l2_l1_messages_hash: SszBytes32
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
    tx_froms_hash: SszBytes32


class SszL2ExecutionProof(Container):
    # An already-proven l2-execution proof, as recursively verified by the
    # rollup guest: carries its own `proof` bytes (a guest's own output never
    # carries `proof` — the prover layer attaches it).
    public_inputs: SszL2ExecutionProofPublicInput
    start_block_number: uint64
    proof: ByteList[MAX_PROOF_BYTES]
    l2_l1_messages: List[SszBytes32, MAX_L2_L1_MESSAGES_PER_EXEC_PROOF]
    tx_froms: List[SszAddress, MAX_TX_FROMS_PER_EXEC_PROOF]
    filtered_addresses: List[SszAddress, MAX_FILTERED_ADDRESSES_PER_EXEC_PROOF]


class SszVerifiableL2ExecutionProof(Container):
    proof: SszL2ExecutionProof
    program_vk: SszBytes32


# ── Logical dataclass -> SSZ view converters ─────────────────────────────────


def _ssz_chain_config(config: ChainConfig) -> SszChainConfig:
    return SszChainConfig(
        l2_message_service_address=bytes(config.l2_message_service_address),
        coinbase=bytes(config.coinbase),
        chain_id=int(config.chain_id),
    )


def _ssz_forced_transaction_witness(ftx: ForcedTransactionWitness) -> SszForcedTransactionWitness:
    return SszForcedTransactionWitness(
        number=int(ftx.number),
        signed_tx_rlp=bytes(ftx.signed_tx_rlp),
        acceptance=int(ftx.acceptance.value),
        deadline=int(ftx.deadline),
    )


def _ssz_lineth_payload_input(payload: LinethPayloadInput) -> SszLinethPayloadInput:
    return SszLinethPayloadInput(
        stateless_input_ssz=bytes(payload.stateless_input_ssz),
        forced_transactions=[
            _ssz_forced_transaction_witness(ftx)
            for ftx in payload.rollup_extension.forced_transactions
        ],
    )


def _ssz_l2_execution_input(private_input: L2ExecutionProofPrivateInput) -> SszL2ExecutionProofPrivateInput:
    return SszL2ExecutionProofPrivateInput(
        parent_ftx_rolling_hash=bytes(private_input.parent_ftx_rolling_hash),
        parent_last_processed_ftx_number=int(private_input.parent_last_processed_ftx_number),
        chain_config=_ssz_chain_config(private_input.chain_config),
        payloads=[_ssz_lineth_payload_input(p) for p in private_input.payloads],
    )


def _ssz_l2_execution_public_input(pi: L2ExecutionProofPublicInput) -> SszL2ExecutionProofPublicInput:
    return SszL2ExecutionProofPublicInput(
        parent_block_hash=bytes(pi.parent_block_hash),
        end_block_hash=bytes(pi.end_block_hash),
        end_block_number=int(pi.end_block_number),
        end_block_timestamp=int(pi.end_block_timestamp),
        l2_l1_messages_hash=bytes(pi.l2_l1_messages_hash),
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
        tx_froms_hash=bytes(pi.tx_froms_hash),
    )


def _ssz_l2_execution_proof(proof: L2ExecutionProof) -> SszL2ExecutionProof:
    return SszL2ExecutionProof(
        public_inputs=_ssz_l2_execution_public_input(proof.public_inputs),
        start_block_number=int(proof.start_block_number),
        proof=bytes(proof.proof),
        l2_l1_messages=[bytes(h) for h in proof.l2_l1_messages],
        tx_froms=[bytes(a) for a in proof.tx_froms],
        filtered_addresses=[bytes(a) for a in proof.filtered_addresses],
    )


def _ssz_verifiable_l2_execution_proof(
    verifiable: VerifiableL2ExecutionProof,
) -> SszVerifiableL2ExecutionProof:
    return SszVerifiableL2ExecutionProof(
        proof=_ssz_l2_execution_proof(verifiable.proof),
        program_vk=bytes(verifiable.program_vk),
    )


# ── SSZ view -> logical dataclass converters ─────────────────────────────────


def _chain_config_from_view(view: Any) -> ChainConfig:
    return ChainConfig(
        l2_message_service_address=Address(bytes(view.l2_message_service_address)),
        coinbase=Address(bytes(view.coinbase)),
        chain_id=U64(int(view.chain_id)),
    )


def _forced_transaction_witness_from_view(view: Any) -> ForcedTransactionWitness:
    try:
        acceptance = ForcedTransactionAcceptance(int(view.acceptance))
    except ValueError as exc:
        raise InvalidSsz(
            f"SszForcedTransactionWitness.acceptance: invalid ForcedTransactionAcceptance value {int(view.acceptance)}"
        ) from exc
    return ForcedTransactionWitness(
        number=U64(int(view.number)),
        signed_tx_rlp=bytes(view.signed_tx_rlp),
        acceptance=acceptance,
        deadline=U64(int(view.deadline)),
    )


def _lineth_payload_input_from_view(view: Any) -> LinethPayloadInput:
    return LinethPayloadInput(
        stateless_input_ssz=bytes(view.stateless_input_ssz),
        rollup_extension=LinethRollupExtension(
            forced_transactions=[
                _forced_transaction_witness_from_view(ftx) for ftx in view.forced_transactions
            ],
        ),
    )


def _l2_execution_input_from_view(view: Any) -> L2ExecutionProofPrivateInput:
    return L2ExecutionProofPrivateInput(
        parent_ftx_rolling_hash=Hash32(bytes(view.parent_ftx_rolling_hash)),
        parent_last_processed_ftx_number=U64(int(view.parent_last_processed_ftx_number)),
        payloads=[_lineth_payload_input_from_view(p) for p in view.payloads],
        chain_config=_chain_config_from_view(view.chain_config),
    )


def _l2_execution_public_input_from_view(view: Any) -> L2ExecutionProofPublicInput:
    return L2ExecutionProofPublicInput(
        parent_block_hash=Hash32(bytes(view.parent_block_hash)),
        end_block_hash=Hash32(bytes(view.end_block_hash)),
        end_block_number=U64(int(view.end_block_number)),
        end_block_timestamp=U64(int(view.end_block_timestamp)),
        l2_l1_messages_hash=Hash32(bytes(view.l2_l1_messages_hash)),
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
        tx_froms_hash=Hash32(bytes(view.tx_froms_hash)),
    )


def _l2_execution_proof_from_view(view: Any) -> L2ExecutionProof:
    return L2ExecutionProof(
        public_inputs=_l2_execution_public_input_from_view(view.public_inputs),
        start_block_number=U64(int(view.start_block_number)),
        proof=bytes(view.proof),
        l2_l1_messages=[Hash32(bytes(h)) for h in view.l2_l1_messages],
        tx_froms=[Address(bytes(a)) for a in view.tx_froms],
        filtered_addresses=[Address(bytes(a)) for a in view.filtered_addresses],
    )


def _verifiable_l2_execution_proof_from_view(view: Any) -> VerifiableL2ExecutionProof:
    return VerifiableL2ExecutionProof(
        proof=_l2_execution_proof_from_view(view.proof),
        program_vk=Hash32(bytes(view.program_vk)),
    )


# ── Extended guest input: dataclass <-> framed SSZ bytes ─────────────────────


def encode_l2_execution_input(private_input: L2ExecutionProofPrivateInput) -> bytes:
    """Encode an `L2ExecutionProofPrivateInput` into framed SSZ bytes (0x0002 schema id)."""
    return _frame(
        L2_EXECUTION_INPUT_SCHEMA_ID,
        _ssz_l2_execution_input(private_input).encode_bytes(),
    )


def decode_l2_execution_input_ssz(data: bytes) -> L2ExecutionProofPrivateInput:
    """
    Decode framed SSZ bytes into an `L2ExecutionProofPrivateInput`. Strict:
    rejects a wrong schema id, truncated bytes, trailing bytes, or
    non-canonical SSZ.
    """
    payload = _strip_frame(data, L2_EXECUTION_INPUT_SCHEMA_ID, "l2-execution input")
    return _l2_execution_input_from_view(_strict_decode(payload, SszL2ExecutionProofPrivateInput))


# ── Extended guest output: hash-only framed wire ─────────────────────────────

# The 0x0003 body is exactly one keccak256 hash.
_OUTPUT_BODY_SIZE = 32


def encode_l2_execution_public_inputs_bytes(pi: L2ExecutionProofPublicInput) -> bytes:
    """
    SSZ-encode the plain 16-field public-input tuple to its fixed 368-byte
    wire representation — the preimage of the hash the 0x0003 output frame
    carries, exposed for verifiers/provers and off-chain inspection.
    """
    return _ssz_l2_execution_public_input(pi).encode_bytes()


def encode_l2_execution_output(proof: L2ExecutionProof) -> bytes:
    """
    Encode the extended l2-execution guest's own output into its framed wire
    bytes (0x0003 schema id): `schema_id || keccak256(ssz(public_inputs))`,
    34 bytes total. Every `proof` field other than `public_inputs` is
    off-chain data with no place on this wire (see module docstring).
    """
    return _frame(
        L2_EXECUTION_OUTPUT_SCHEMA_ID,
        keccak256(encode_l2_execution_public_inputs_bytes(proof.public_inputs)),
    )


def decode_l2_execution_output_ssz(data: bytes) -> Hash32:
    """
    Decode a framed l2-execution output into the public-inputs hash it
    carries. The body is `keccak256` of the SSZ-encoded public-input tuple —
    irreversible, so no dataclass is reconstructed. Strict: rejects a wrong
    schema id, a truncated body, or trailing bytes.
    """
    payload = _strip_frame(data, L2_EXECUTION_OUTPUT_SCHEMA_ID, "l2-execution output")
    if len(payload) != _OUTPUT_BODY_SIZE:
        raise InvalidSsz(
            f"l2-execution output: body must be exactly {_OUTPUT_BODY_SIZE} bytes, got {len(payload)}"
        )
    return Hash32(payload)
