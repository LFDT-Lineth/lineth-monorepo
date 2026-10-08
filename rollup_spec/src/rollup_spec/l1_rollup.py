from dataclasses import dataclass, field
from typing import Dict, List, Set

from ethereum.crypto.hash import Hash32, keccak256
from ethereum.state import Address
from ethereum_types.numeric import U64

from .rollup import DataRollingHashWitness


def _encode_tail_count(tail_count: int) -> bytes:
    """32-byte big-endian encoding of a blob tail byte count, matching how the
    L1 contract ABI-packs a `uint256` into a keccak256 preimage."""
    return tail_count.to_bytes(32, "big")


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

    `current_finalized_position_commitment` is the enforced-boundary variant
    (§3.6, §8 Q2): `keccak256(endDataRollingHash || encode_tail_count(finalDataTailDiscard))`, sealed
    into the same slot that used to hold a plain shnarf — zero additional
    storage. The next finalization supplies the parent `(data_rolling_hash, tail_count)` pair
    as calldata (`finalize_rollup`'s `parent_data_rolling_hash`/`parent_data_tail_take` params); the
    contract verifies the preimage against this commitment before checking
    exact continuity.
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
    # Anchor storage (§3.6): a plain set of anchored dataRollingHash values. Execution
    # continuity no longer travels with the DA accumulator (§2.4), so there
    # is no per-dataRollingHash lastBlockHash to track anymore — just membership.
    anchored_data_rolling_hashes: Set[Hash32] = field(default_factory=set)
    l2_merkle_roots_depths: Dict[Hash32, int] = field(default_factory=dict)
    # Security-council-managed identities of the guest programs approved for finalization.
    approved_program_ids: Set[Hash32] = field(default_factory=set)


@dataclass
class FinalizationPublicInput:
    """Final aggregation PI. Program IDs identify approved guest programs on L1."""
    end_block_number: U64
    end_block_timestamp: U64
    parent_l1_l2_bridge_rolling_hash: Hash32
    parent_l1_l2_bridge_rolling_hash_message_number: U64
    end_l1_l2_bridge_rolling_hash: Hash32
    end_l1_l2_bridge_rolling_hash_message_number: U64
    dynamic_chain_config_hash: Hash32
    parent_ftx_rolling_hash: Hash32
    parent_ftx_number: U64
    end_ftx_rolling_hash: Hash32
    end_processed_ftx_number: U64
    parent_data_rolling_hash: Hash32
    end_data_rolling_hash: Hash32
    parent_block_hash: Hash32
    end_block_hash: Hash32
    parent_data_tail_take: int
    final_data_tail_discard: int
    l2_l1_tree_depth: int
    l2_l1_roots: List[Hash32] = field(default_factory=list)
    filtered_addresses: List[Address] = field(default_factory=list)
    program_ids: List[Hash32] = field(default_factory=list)
    l2_messaging_blocks_offsets: List[int] = field(default_factory=list)


@dataclass
class FinalizationSubmission:
    """
    The rollup-aggregation guest output as submitted to the L1 finalization
    call. It is the guest output plus the `proof` bytes: the
    `public_inputs` tuple. L2-to-L1 roots and filtered addresses are bound
    directly in the public inputs.

    Guest/prover boundary: the aggregation guest emits `public_inputs`; `proof`
    is attached by the zkVM/prover layer above and is a placeholder (`b""`) in
    this reference (see `run_rollup_aggregation_guest`).
    `public_inputs.l2_messaging_blocks_offsets` is bound inside the PI and emitted by L1.

    The combined program-ID list lives inside `public_inputs.program_ids`, binding
    its order to the proof. L1 checks every ID against `approved_program_ids`.
    """
    public_inputs: FinalizationPublicInput
    proof: bytes


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


def finalize_rollup(
    state: LinethRollupState,
    submission: FinalizationSubmission,
    parent_data_rolling_hash: Hash32,
    parent_data_tail_take: int,
) -> bytes:
    """
    `parent_data_rolling_hash` / `parent_data_tail_take` are the previously-finalized end boundary,
    supplied as calldata so the contract can open the stored position
    commitment (§3.6, enforced variant) — the caller reads them from the
    prior finalization's event/return value rather than the contract storing
    them in the clear.
    """
    pi = submission.public_inputs

    if not verify_rollup_aggregation_snark(submission.proof, pi):
        raise Exception("invalid rollup-aggregation proof")
    previous_messaging_offset = 0
    finalized_block_count = int(pi.end_block_number) - int(state.current_l2_block_number)
    for offset in pi.l2_messaging_blocks_offsets:
        if (
            type(offset) is not int
            or not previous_messaging_offset < offset <= finalized_block_count
            or offset > 0xFFFF
        ):
            raise Exception("invalid finalized messaging block offset")
        previous_messaging_offset = offset
    if keccak256(parent_data_rolling_hash + _encode_tail_count(parent_data_tail_take)) != state.current_finalized_position_commitment:
        raise Exception("parentDataRollingHash/parentDataTailTake do not match the finalized position commitment")
    if pi.parent_data_rolling_hash != parent_data_rolling_hash:
        raise Exception("parentDataRollingHash does not match the finalized position")
    if pi.parent_data_tail_take != parent_data_tail_take:
        raise Exception("parentDataTailTake does not match the finalized position")
    if pi.end_data_rolling_hash not in state.anchored_data_rolling_hashes:
        raise Exception("endDataRollingHash was not anchored by a chunk submission")
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

    for root in pi.l2_l1_roots:
        state.l2_merkle_roots_depths[root] = pi.l2_l1_tree_depth

    for address in pi.filtered_addresses:
        if address not in state.sanctioned_addresses:
            raise Exception("filtered address is not sanctioned")

    for program_id in pi.program_ids:
        if program_id not in state.approved_program_ids:
            raise Exception("program ID is not approved")

    state.current_finalized_position_commitment = keccak256(
        pi.end_data_rolling_hash + _encode_tail_count(pi.final_data_tail_discard)
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

    return b"".join(offset.to_bytes(2, "big") for offset in pi.l2_messaging_blocks_offsets)


def verify_rollup_aggregation_snark(proof: bytes, public_inputs: FinalizationPublicInput) -> bool:
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
