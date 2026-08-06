from dataclasses import dataclass, field
from typing import Dict, List, Set

from ethereum.crypto.hash import Hash32, keccak256
from ethereum.state import Address
from ethereum_types.numeric import U64

from .l2_execution import hash_address_list, hash_hash_list
from .rollup import L2_L1_TREE_DEPTH, DrhWitness, RollupPublicInput


def _encode_offset(offset: int) -> bytes:
    """32-byte big-endian encoding of a stream byte offset, matching how the
    L1 contract ABI-packs a `uint256` into a keccak256 preimage."""
    return offset.to_bytes(32, "big")


@dataclass
class PlonkVerifier:
    """
    Model of the on-chain `IPlonkVerifier` interface (see
    `contracts/src/verifiers/interfaces/IPlonkVerifier.sol`).

    The chain-configuration preimage (`chainId`, `baseFee`, `coinbase`,
    `l2MessageServiceAddress`) is passed to the verifier at deploy time as
    a `ChainConfigurationParameter[]` array of named bytes32 values (see
    `contracts/deploy/01_deploy_PlonkVerifier.ts`); the constructor hashes
    those values and stores ONLY the digest in `bytes32 immutable
    CHAIN_CONFIGURATION`, then emits the full preimage in the
    `ChainConfigurationSet` event. The L1 `LinethRollupBase` reads the
    digest at finalization time via `getChainConfiguration()`; changing
    the chain configuration requires deploying a new verifier and pointing
    the rollup at it via `setVerifierAddress`. The preimage is therefore
    auditable on-chain (via the deploy event + the verifier's constructor
    args) but is not held in any contract's runtime storage.
    """
    chain_configuration_hash: Hash32

    def get_chain_configuration(self) -> Hash32:
        return self.chain_configuration_hash


@dataclass
class LinethRollupState:
    """
    L1 `LinethRollup` storage relevant to proof finalization.

    Note that `dynamicChainConfigHash` is NOT a field of this state — it
    lives in the verifier as an immutable bytes32, and is read via
    `verifier.get_chain_configuration()` (modelled by the `PlonkVerifier`
    field below).

    `current_finalized_position_commitment` is the enforced-offset variant
    (§3.6, §8 Q2): `keccak256(endDrh || encode_offset(endOffset))`, sealed
    into the same slot that used to hold a plain shnarf — zero additional
    storage. The next finalization supplies the previous `(drh, offset)` pair
    as calldata (`finalize_rollup`'s `prev_drh`/`prev_offset` params); the
    contract verifies the preimage against this commitment before applying
    the continuity disjunction.
    """
    current_finalized_position_commitment: Hash32
    current_finalized_last_block_hash: Hash32
    current_l2_block_number: U64
    current_l2_block_timestamp: U64
    current_finalized_l1_l2_bridge_rolling_hash: Hash32
    current_finalized_l1_l2_bridge_rolling_hash_message_number: U64
    current_finalized_ftx_rolling_hash: Hash32
    current_finalized_processed_ftx_number: U64
    verifier: PlonkVerifier
    l1_l2_rolling_hashes: Dict[U64, Hash32] = field(default_factory=dict)
    ftx_rolling_hashes: Dict[U64, Hash32] = field(default_factory=dict)
    ftx_deadlines: Dict[U64, U64] = field(default_factory=dict)
    sanctioned_addresses: Set[Address] = field(default_factory=set)
    # Anchor storage (§3.6): a plain set of anchored DRH values. Execution
    # continuity no longer travels with the DA accumulator (§2.4), so there
    # is no per-DRH lastBlockHash to track anymore — just membership.
    anchored_drhs: Set[Hash32] = field(default_factory=set)
    l2_merkle_roots_depths: Dict[Hash32, int] = field(default_factory=dict)
    # The single, combined security-council-managed approved-VK list
    # (§ProgramVK anchoring). Exec and rollup VKs are NOT distinguished on L1 —
    # a finalization's single `public_inputs.program_vks` list is checked against
    # this one set. On-chain this is managed by an add/remove setter analogous
    # to `setVerifierAddress` (replace on soundness bug, add on non-soundness
    # guest update, periodic cleanup); not modelled as a method here.
    approved_vks: Set[Hash32] = field(default_factory=set)


@dataclass
class FinalizationSubmission:
    """
    The rollup-aggregation guest output as submitted to the L1 finalization
    call. It is the guest output plus the `proof` bytes: the 20-field
    `public_inputs` tuple and the revealed preimages L1 needs as calldata —
    `l2_l1_roots` (preimage of `l2L1BridgeTransactionTree`) and
    `filtered_addresses` (preimage of `filteredAddressesHash`).

    Guest/prover boundary: the aggregation guest emits `public_inputs` and the
    preimage lists; `proof` is attached by the zkVM/prover layer above and is a
    placeholder (`b""`) in this reference (see `run_rollup_aggregation_guest`).
    `l2_messaging_blocks_offsets` is carried for the L1 calldata shape but is
    not yet consumed by `finalize_rollup`.

    The single combined program-VK list (§ProgramVK anchoring) lives inside
    `public_inputs.program_vks` so its order is bound to the proof; it is NOT a
    separate submission field. `finalize_rollup` checks every entry against the
    L1 `approved_vks` set.
    """
    public_inputs: RollupPublicInput
    proof: bytes
    l2_l1_roots: List[Hash32]
    filtered_addresses: List[Address]
    l2_messaging_blocks_offsets: List[int] = field(default_factory=list)


def anchor_chunk_submission(
    state: LinethRollupState,
    parent_drh: Hash32,
    chunk_hash: Hash32,
) -> Hash32:
    """
    Anchor one submitted chunk (§3.6): fold `chunk_hash` into the DRH chain
    and record the result as anchored. Called once per chunk in a submission
    transaction (`submitBlobs(bytes32 _parentDrh, bytes32 _finalDrh)` folds
    `blobhash(i)` for each `i` this same way on-chain; the caller loops over
    multiple chunks in one submission itself).
    """
    end_drh = DrhWitness(parent_drh, chunk_hash).hash()
    state.anchored_drhs.add(end_drh)
    return end_drh


def finalize_rollup(
    state: LinethRollupState,
    submission: FinalizationSubmission,
    prev_drh: Hash32,
    prev_offset: int,
) -> None:
    """
    `prev_drh` / `prev_offset` are the previously-finalized end position,
    supplied as calldata so the contract can open the stored position
    commitment (§3.6, enforced variant) — the caller reads them from the
    prior finalization's event/return value rather than the contract storing
    them in the clear.
    """
    pi = submission.public_inputs

    if not verify_rollup_aggregation_snark(submission.proof, pi):
        raise Exception("invalid rollup-aggregation proof")
    if keccak256(prev_drh + _encode_offset(prev_offset)) != state.current_finalized_position_commitment:
        raise Exception("prevDrh/prevOffset do not match the finalized position commitment")
    if pi.parent_drh != prev_drh:
        raise Exception("parentDrh does not match the finalized position")
    if not (pi.start_offset == prev_offset or pi.start_offset == 0):
        raise Exception("startOffset neither continues the finalized position nor is a fresh start")
    if pi.end_drh not in state.anchored_drhs:
        raise Exception("endDrh was not anchored by a chunk submission")
    if pi.parent_block_hash != state.current_finalized_last_block_hash:
        raise Exception("parentBlockHash does not match the currently finalized block hash")
    if pi.parent_l1_l2_bridge_rolling_hash != state.current_finalized_l1_l2_bridge_rolling_hash:
        raise Exception("L1-to-L2 rolling hash continuity mismatch")
    if (
        pi.parent_l1_l2_bridge_rolling_hash_message_number !=
        state.current_finalized_l1_l2_bridge_rolling_hash_message_number
    ):
        raise Exception("L1-to-L2 rolling hash message number continuity mismatch")
    if _l1_l2_rolling_hash_at(state, pi.end_l1_l2_bridge_rolling_hash_message_number) != (
        pi.end_l1_l2_bridge_rolling_hash
    ):
        raise Exception("L1-to-L2 rolling hash does not match L1 storage")
    if pi.dynamic_chain_config_hash != state.verifier.get_chain_configuration():
        # The verifier holds the chain-configuration hash as an immutable
        # bytes32 set at its deploy time; the full preimage (chainId,
        # baseFee, coinbase, l2MessageServiceAddress) is auditable via the
        # `ChainConfigurationSet` event the verifier emitted at deploy.
        raise Exception("dynamic chain config hash mismatch")
    if pi.parent_ftx_rolling_hash != state.current_finalized_ftx_rolling_hash:
        raise Exception("FTX rolling hash continuity mismatch")
    if pi.parent_processed_ftx_number != state.current_finalized_processed_ftx_number:
        raise Exception("processed FTX number continuity mismatch")
    if pi.end_processed_ftx_number < state.current_finalized_processed_ftx_number:
        raise Exception("endProcessedFtxNumber cannot decrease")
    if _ftx_rolling_hash_at(state, pi.end_processed_ftx_number) != pi.end_ftx_rolling_hash:
        raise Exception("FTX rolling hash does not match L1 storage")

    _check_forced_transaction_deadlines(
        state,
        pi.end_block_number,
        pi.end_processed_ftx_number,
    )

    if hash_hash_list(submission.l2_l1_roots) != pi.l2_l1_bridge_transaction_tree:
        raise Exception("submitted L2-to-L1 roots do not match public input")
    for root in submission.l2_l1_roots:
        state.l2_merkle_roots_depths[root] = L2_L1_TREE_DEPTH

    if hash_address_list(submission.filtered_addresses) != pi.filtered_addresses_hash:
        raise Exception("submitted filtered addresses do not match public input")
    for address in submission.filtered_addresses:
        if address not in state.sanctioned_addresses:
            raise Exception("filtered address is not sanctioned")

    # §ProgramVK anchoring: every guest verified beneath this finalization must
    # be on the single combined approved-VK list, or L1 rejects the finalization
    # (e.g. an operator swapping in an unapproved guest). Exec and rollup VKs are
    # not distinguished — they arrive as one `program_vks` list. `program_vks` is
    # the canonical sorted-distinct set, so this membership scan is
    # order-independent (each entry checked against `approved_vks`).
    for vk in pi.program_vks:
        if vk not in state.approved_vks:
            raise Exception("program VK is not approved")

    state.current_finalized_position_commitment = keccak256(
        pi.end_drh + _encode_offset(pi.end_offset)
    )
    state.current_finalized_last_block_hash = pi.end_block_hash
    state.current_l2_block_number = pi.end_block_number
    state.current_l2_block_timestamp = pi.end_block_timestamp
    state.current_finalized_l1_l2_bridge_rolling_hash = pi.end_l1_l2_bridge_rolling_hash
    state.current_finalized_l1_l2_bridge_rolling_hash_message_number = (
        pi.end_l1_l2_bridge_rolling_hash_message_number
    )
    state.current_finalized_ftx_rolling_hash = pi.end_ftx_rolling_hash
    state.current_finalized_processed_ftx_number = pi.end_processed_ftx_number


def verify_rollup_aggregation_snark(proof: bytes, public_inputs: RollupPublicInput) -> bool:
    return True


verify_aggregation_snark = verify_rollup_aggregation_snark


def _l1_l2_rolling_hash_at(state: LinethRollupState, message_number: U64) -> Hash32:
    if message_number == state.current_finalized_l1_l2_bridge_rolling_hash_message_number:
        return state.current_finalized_l1_l2_bridge_rolling_hash
    if message_number not in state.l1_l2_rolling_hashes:
        raise Exception("missing L1-to-L2 rolling hash for message number")
    return state.l1_l2_rolling_hashes[message_number]


def _ftx_rolling_hash_at(state: LinethRollupState, ftx_number: U64) -> Hash32:
    if ftx_number == state.current_finalized_processed_ftx_number:
        return state.current_finalized_ftx_rolling_hash
    if ftx_number not in state.ftx_rolling_hashes:
        raise Exception("missing FTX rolling hash for forced transaction number")
    return state.ftx_rolling_hashes[ftx_number]


def _check_forced_transaction_deadlines(
    state: LinethRollupState,
    end_block_number: U64,
    last_processed_ftx_number: U64,
) -> None:
    for ftx_number, deadline in state.ftx_deadlines.items():
        if deadline <= end_block_number and ftx_number > last_processed_ftx_number:
            raise Exception("cannot finalize past an unprocessed forced transaction deadline")
