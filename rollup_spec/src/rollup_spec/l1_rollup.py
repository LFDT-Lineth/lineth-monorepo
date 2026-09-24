from dataclasses import dataclass, field
from typing import Dict, List, Set

from ethereum.crypto.hash import Hash32, keccak256
from ethereum.state import Address
from ethereum_types.numeric import U64

from .l2_execution import hash_address_list, hash_digest_list
from .rollup import L2_L1_TREE_DEPTH, DataRollingHashWitness, RollupPublicInput

ZERO_HASH32 = Hash32(b"\x00" * 32)


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

    The finalized DA stream position is tracked DIRECTLY and readably on-chain as the
    plain pair `current_data_rolling_hash` / `current_data_availability_offset`
    (`currentDataRollingHash` / `currentDataAvailabilityOffset` on-chain). There is no
    opaque position commitment to open: the coordinator reads the live position off the
    contract to build the next submission/finalization, and finalization asserts the
    supplied `parent_data_rolling_hash` / `start_offset` equal these stored values exactly.

    `current_finalized_shnarf_deprecated` is the retained legacy `currentFinalizedShnarf`
    slot (the pre-upgrade contract's live finalized shnarf). It is consumed exactly once by
    `reinitialize_linea_rollup_v10` (the legacy-shnarf migration) and wiped to `ZERO_HASH32`
    there; `finalize_rollup` never reads or writes it.
    """
    current_data_rolling_hash: Hash32
    current_data_availability_offset: int
    current_l2_block_number: U64
    current_l2_block_timestamp: U64
    current_finalized_l1_l2_bridge_rolling_hash: Hash32
    current_finalized_l1_l2_bridge_rolling_hash_message_number: U64
    current_finalized_ftx_rolling_hash: Hash32
    current_finalized_processed_ftx_number: U64
    verifier: PlonkVerifier
    current_finalized_shnarf_deprecated: Hash32 = ZERO_HASH32
    block_hashes: Dict[U64, Hash32] = field(default_factory=dict)
    # Legacy per-block-number state roots (`stateRootHashes` on-chain), consulted only on
    # the state-root → block-hash migration path: when `block_hashes` has no entry for the
    # last finalized block, finalization falls back to matching `parent_state_root_hash`.
    state_root_hashes: Dict[U64, Hash32] = field(default_factory=dict)
    l1_l2_rolling_hashes: Dict[U64, Hash32] = field(default_factory=dict)
    ftx_rolling_hashes: Dict[U64, Hash32] = field(default_factory=dict)
    ftx_deadlines: Dict[U64, U64] = field(default_factory=dict)
    sanctioned_addresses: Set[Address] = field(default_factory=set)
    # Anchor storage (§3.6): a plain set of anchored dataRollingHash values. Execution
    # continuity no longer travels with the DA accumulator (§2.4), so there
    # is no per-dataRollingHash lastBlockHash to track anymore — just membership.
    anchored_data_rolling_hashes: Set[Hash32] = field(default_factory=set)
    l2_merkle_roots_depths: Dict[Hash32, int] = field(default_factory=dict)
    # The single, combined security-council-managed approved-VK list
    # (§ProgramVK anchoring; `verifierKeys` on-chain). Exec and rollup VKs are NOT
    # distinguished on L1 — a finalization's single `public_inputs.program_vks` list is
    # checked against this one set. On-chain this is managed by `setVerifierKeys` /
    # `unsetVerifierKeys`; not modelled as a method here.
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

    `parent_state_root_hash` is the legacy continuity value (`FinalizationDataV5.parentStateRootHash`),
    consulted ONLY on the state-root → block-hash migration path: when `block_hashes` has no
    entry for the last finalized block, finalization matches it against `state_root_hashes`
    instead of `parent_block_hash`. It is ignored once the block-hash path is active.
    """
    public_inputs: RollupPublicInput
    proof: bytes
    l2_l1_roots: List[Hash32]
    filtered_addresses: List[Address]
    parent_state_root_hash: Hash32 = ZERO_HASH32
    l2_messaging_blocks_offsets: List[int] = field(default_factory=list)


def anchor_chunk_submission(
    state: LinethRollupState,
    parent_data_rolling_hash: Hash32,
    chunk_hash: Hash32,
) -> Hash32:
    """
    Anchor one submitted chunk (§3.6): fold `chunk_hash` into the dataRollingHash chain
    and record the result as anchored. Called once per chunk in a submission
    transaction (`submitBlobs(bytes32 _parentDataRollingHash, bytes32 _finalDataRollingHash)` folds
    `blobhash(i)` for each `i` this same way on-chain; the caller loops over
    multiple chunks in one submission itself).
    """
    end_data_rolling_hash = DataRollingHashWitness(parent_data_rolling_hash, chunk_hash).hash()
    state.anchored_data_rolling_hashes.add(end_data_rolling_hash)
    return end_data_rolling_hash


def seed_genesis_position(
    state: LinethRollupState,
    initial_block_hash: Hash32,
    initial_l2_block_number: U64,
) -> Hash32:
    """
    Fresh-network genesis seeding (`__LinethRollup_init`, new testnets / local / CI).

    The genesis DA stream position is seeded deterministically from the genesis block
    hash — `current_data_rolling_hash = keccak256(EMPTY_HASH || initialBlockHash)` (the
    `EfficientLeftRightKeccak._efficientKeccak(EMPTY_HASH, initialBlockHash)` fold) — and
    anchored into `_dataRollingHashExists` so the first submission can chain from it.
    `current_data_availability_offset` stays `0` (fresh-start) and
    `current_finalized_shnarf_deprecated` stays `ZERO_HASH32` (no legacy shnarf to migrate).
    The genesis block hash is also anchored into `block_hashes`.
    """
    if initial_block_hash == ZERO_HASH32:
        raise Exception("initialBlockHash cannot be the zero hash")
    genesis_data_rolling_hash = keccak256(ZERO_HASH32 + initial_block_hash)
    state.current_data_rolling_hash = genesis_data_rolling_hash
    state.current_data_availability_offset = 0
    state.anchored_data_rolling_hashes.add(genesis_data_rolling_hash)
    state.block_hashes[initial_l2_block_number] = initial_block_hash
    return genesis_data_rolling_hash


def reinitialize_linea_rollup_v10(state: LinethRollupState) -> Hash32:
    """
    Legacy-shnarf migration (`LinethRollup.reinitializeLineaRollupV10()`, in-place upgrades).

    One-way bridge from the legacy shnarf model to the blob-spanning dataRollingHash model.
    It trusts the on-chain `current_finalized_shnarf_deprecated` slot directly (that value was
    itself the proven output of the prior contract version's `finalizeBlocks`) and reinterprets
    it as the live end dataRollingHash: the value becomes `current_data_rolling_hash`, is
    anchored into the dataRollingHash membership set so post-upgrade submissions chain from it,
    and the legacy slot is wiped to `ZERO_HASH32` (never written again). The migration runs
    exactly once per proxy (enforced on-chain by `reinitializer(10)`); on any real in-place
    upgrade the slot can never be `ZERO_HASH32`, so no emptiness guard is needed.
    """
    migrated_data_rolling_hash = state.current_finalized_shnarf_deprecated
    state.current_data_rolling_hash = migrated_data_rolling_hash
    state.anchored_data_rolling_hashes.add(migrated_data_rolling_hash)
    state.current_finalized_shnarf_deprecated = ZERO_HASH32
    return migrated_data_rolling_hash


def finalize_rollup(
    state: LinethRollupState,
    submission: FinalizationSubmission,
) -> None:
    """Apply a finalized rollup range after checking stored DA and block continuity."""
    pi = submission.public_inputs

    if not verify_rollup_aggregation_snark(submission.proof, pi):
        raise Exception("invalid rollup-aggregation proof")
    if pi.parent_data_rolling_hash != state.current_data_rolling_hash:
        raise Exception("parentDataRollingHash does not match the current data rolling hash")
    if pi.start_offset != state.current_data_availability_offset:
        raise Exception("startOffset does not match the current data availability offset")
    if pi.end_data_rolling_hash not in state.anchored_data_rolling_hashes:
        raise Exception("endDataRollingHash was not anchored by a chunk submission")

    # Execution rooting, with the state-root → block-hash migration path (§5.4):
    # `blockHashes[lastFinalizedBlock]` is the authoritative anchor on the new path.
    # EMPTY_HASH (absent) signals the migration path — the parent was committed under the
    # old state-root-hash model, so the caller supplies `parentBlockHash == EMPTY_HASH` and
    # the contract instead matches the legacy `stateRootHashes[lastFinalizedBlock]`.
    parent_block_hash = state.block_hashes.get(state.current_l2_block_number, ZERO_HASH32)
    if parent_block_hash == ZERO_HASH32:
        if pi.parent_block_hash != ZERO_HASH32:
            raise Exception("parentBlockHash must be EMPTY_HASH on the migration path")
        parent_state_root_hash = state.state_root_hashes.get(state.current_l2_block_number, ZERO_HASH32)
        if parent_state_root_hash == ZERO_HASH32 or parent_state_root_hash != submission.parent_state_root_hash:
            raise Exception("parentStateRootHash does not match the stored state root hash")
    else:
        if pi.parent_block_hash != parent_block_hash:
            raise Exception("parentBlockHash does not match the current block hash")

    if pi.end_block_hash == ZERO_HASH32:
        raise Exception("finalBlockHash cannot be the zero hash")
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
    if pi.parent_ftx_number != state.current_finalized_processed_ftx_number:
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

    if hash_digest_list(submission.l2_l1_roots) != pi.l2_l1_bridge_transaction_tree:
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

    state.current_data_rolling_hash = pi.end_data_rolling_hash
    state.current_data_availability_offset = pi.end_offset
    state.block_hashes[pi.end_block_number] = pi.end_block_hash
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
