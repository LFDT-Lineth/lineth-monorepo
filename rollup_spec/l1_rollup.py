from dataclasses import dataclass, field
from typing import Dict, List, Set

from ethereum.crypto.hash import Hash32
from ethereum.state import Address
from ethereum_types.numeric import U64

from .l2_execution import hash_address_list, hash_hash_list
from .rollup import L2_L1_TREE_DEPTH, RollupPublicInput, ShnarfWitness


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
    `ChainConfigurationSet` event. The L1 `LineaRollupBase` reads the
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
class LineaRollupState:
    """
    L1 `LineaRollup` storage relevant to proof finalization.

    Note that `dynamicChainConfigHash` is NOT a field of this state — it
    lives in the verifier as an immutable bytes32, and is read via
    `verifier.get_chain_configuration()` (modelled by the `PlonkVerifier`
    field below).
    """
    current_finalized_shnarf: Hash32
    current_finalized_last_block_hash: Hash32
    current_l2_block_number: U64
    current_l2_block_timestamp: U64
    current_finalized_l1_l2_bridge_rolling_hash: Hash32
    current_finalized_l1_l2_bridge_rolling_hash_message_number: U64
    current_finalized_ftx_rolling_hash: Hash32
    current_finalized_last_processed_ftx_number: U64
    verifier: PlonkVerifier
    l1_l2_rolling_hashes: Dict[U64, Hash32] = field(default_factory=dict)
    ftx_rolling_hashes: Dict[U64, Hash32] = field(default_factory=dict)
    ftx_deadlines: Dict[U64, U64] = field(default_factory=dict)
    sanctioned_addresses: Set[Address] = field(default_factory=set)
    submitted_shnarf_last_block_hashes: Dict[Hash32, Hash32] = field(default_factory=dict)
    l2_merkle_roots_depths: Dict[Hash32, int] = field(default_factory=dict)


@dataclass
class FinalizationSubmission:
    """
    Calldata supplied to the L1 finalization call.
    """
    public_inputs: RollupPublicInput
    proof: bytes
    l2_l1_roots: List[Hash32]
    filtered_addresses: List[Address]
    l2_messaging_blocks_offsets: List[int] = field(default_factory=list)


def anchor_blob_submission(
    state: LineaRollupState,
    parent_shnarf: Hash32,
    last_block_hash: Hash32,
    blob_hash: Hash32,
) -> Hash32:
    end_shnarf = ShnarfWitness(parent_shnarf, last_block_hash, blob_hash).hash()
    state.submitted_shnarf_last_block_hashes[end_shnarf] = last_block_hash
    return end_shnarf


def finalize_rollup(state: LineaRollupState, submission: FinalizationSubmission) -> None:
    pi = submission.public_inputs

    if not verify_rollup_aggregation_snark(submission.proof, pi):
        raise Exception("invalid rollup-aggregation proof")
    if pi.parent_shnarf != state.current_finalized_shnarf:
        raise Exception("parentShnarf does not match current finalized shnarf")
    if pi.end_shnarf not in state.submitted_shnarf_last_block_hashes:
        raise Exception("endShnarf was not anchored by a blob submission")
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
    if pi.last_processed_ftx_number < state.current_finalized_last_processed_ftx_number:
        raise Exception("lastProcessedFtxNumber cannot decrease")
    if _ftx_rolling_hash_at(state, pi.last_processed_ftx_number) != pi.end_ftx_rolling_hash:
        raise Exception("FTX rolling hash does not match L1 storage")

    _check_forced_transaction_deadlines(
        state,
        pi.end_block_number,
        pi.last_processed_ftx_number,
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

    state.current_finalized_shnarf = pi.end_shnarf
    state.current_finalized_last_block_hash = state.submitted_shnarf_last_block_hashes[pi.end_shnarf]
    state.current_l2_block_number = pi.end_block_number
    state.current_l2_block_timestamp = pi.end_block_timestamp
    state.current_finalized_l1_l2_bridge_rolling_hash = pi.end_l1_l2_bridge_rolling_hash
    state.current_finalized_l1_l2_bridge_rolling_hash_message_number = (
        pi.end_l1_l2_bridge_rolling_hash_message_number
    )
    state.current_finalized_ftx_rolling_hash = pi.end_ftx_rolling_hash
    state.current_finalized_last_processed_ftx_number = pi.last_processed_ftx_number


def verify_rollup_aggregation_snark(proof: bytes, public_inputs: RollupPublicInput) -> bool:
    return True


verify_aggregation_snark = verify_rollup_aggregation_snark


def _l1_l2_rolling_hash_at(state: LineaRollupState, message_number: U64) -> Hash32:
    if message_number == state.current_finalized_l1_l2_bridge_rolling_hash_message_number:
        return state.current_finalized_l1_l2_bridge_rolling_hash
    if message_number not in state.l1_l2_rolling_hashes:
        raise Exception("missing L1-to-L2 rolling hash for message number")
    return state.l1_l2_rolling_hashes[message_number]


def _ftx_rolling_hash_at(state: LineaRollupState, ftx_number: U64) -> Hash32:
    if ftx_number == state.current_finalized_last_processed_ftx_number:
        return state.current_finalized_ftx_rolling_hash
    if ftx_number not in state.ftx_rolling_hashes:
        raise Exception("missing FTX rolling hash for forced transaction number")
    return state.ftx_rolling_hashes[ftx_number]


def _check_forced_transaction_deadlines(
    state: LineaRollupState,
    end_block_number: U64,
    last_processed_ftx_number: U64,
) -> None:
    for ftx_number, deadline in state.ftx_deadlines.items():
        if deadline <= end_block_number and ftx_number > last_processed_ftx_number:
            raise Exception("cannot finalize past an unprocessed forced transaction deadline")
