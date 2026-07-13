from dataclasses import dataclass
from typing import List, Set

from ethereum.crypto.hash import Hash32
from ethereum.state import Address

from .l1_rollup import FinalizationSubmission
from .l2_execution import hash_address_list, hash_hash_list
from .rollup import RollupProof, RollupPublicInput, recursive_stark_verify


@dataclass
class RollupAggregationProofPrivateInput:
    """
    Logical rollup-aggregation request. The topology is flat across M rollup
    proofs, as specified in Readme.md section 2.3.
    """
    rollup_proofs: List[RollupProof]


def run_rollup_aggregation_guest(
    aggregation_input: RollupAggregationProofPrivateInput,
) -> FinalizationSubmission:
    """
    rollup-aggregation: flat recursion over M rollup proofs with continuity
    checks and merged L2-to-L1 root/address commitments.

    Returns a `FinalizationSubmission`: the guest output (the 14-field
    public-input tuple + the revealed `l2_l1_roots` / `filtered_addresses`
    preimages L1 needs as calldata). `proof` is attached by the zkVM/prover
    layer above and is a placeholder (`b""`) here.
    """
    if len(aggregation_input.rollup_proofs) == 0:
        raise Exception("rollup-aggregation proof must consume at least one rollup proof")

    for proof in aggregation_input.rollup_proofs:
        verify_rollup_proof(proof.program_vk, proof)

    for left, right in zip(aggregation_input.rollup_proofs, aggregation_input.rollup_proofs[1:]):
        assert_rollup_proof_continuity(left, right)

    first_proof = aggregation_input.rollup_proofs[0]
    last_proof = aggregation_input.rollup_proofs[-1]
    merged_l2_l1_roots: List[Hash32] = []
    merged_filtered_addresses: List[Address] = []

    # §ProgramVK anchoring: emit ONE `program_vks` set as a CANONICAL sorted,
    # distinct list — L1 does not distinguish exec vs rollup VKs (single combined
    # `approvedVks` set), and sorting makes the commitment a pure function of the
    # set's contents. The set is the union of every rollup proof's bubbled
    # `public_inputs.program_vks` (the exec VKs it verified) and each proof's own
    # `program_vk`. `rollup_vks` is kept only as internal trace of the distinct
    # rollup `program_vk`s that were verified.
    rollup_vks: List[Hash32] = []  # internal trace only
    seen_rollup_vks: Set[Hash32] = set()
    program_vk_set: Set[Hash32] = set()

    for proof in aggregation_input.rollup_proofs:
        merged_l2_l1_roots.extend(proof.l2_l1_roots)
        merged_filtered_addresses.extend(proof.filtered_addresses)
        if proof.program_vk not in seen_rollup_vks:
            seen_rollup_vks.add(proof.program_vk)
            rollup_vks.append(proof.program_vk)
        # Union in the bubbled exec VKs verified beneath this rollup proof, plus
        # the rollup proof's own VK.
        program_vk_set.update(proof.public_inputs.program_vks)
        program_vk_set.add(proof.program_vk)

    # Canonical set encoding: sorted ascending by byte value (Hash32 is bytes).
    program_vks = sorted(program_vk_set)

    public_inputs = RollupPublicInput(
        end_block_number=last_proof.public_inputs.end_block_number,
        end_block_timestamp=last_proof.public_inputs.end_block_timestamp,
        l2_l1_bridge_transaction_tree=hash_hash_list(merged_l2_l1_roots),
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
        filtered_addresses_hash=hash_address_list(merged_filtered_addresses),
        parent_shnarf=first_proof.public_inputs.parent_shnarf,
        end_shnarf=last_proof.public_inputs.end_shnarf,
        program_vks=program_vks,
    )

    return FinalizationSubmission(
        public_inputs=public_inputs,
        proof=bytes(),  # Placeholder: filled by zkVM prover at layer above
        l2_l1_roots=merged_l2_l1_roots,
        filtered_addresses=merged_filtered_addresses,
        l2_messaging_blocks_offsets=[],  # Not populated from rollup proofs; defaults to empty
    )


def verify_rollup_proof(program_vk: Hash32, proof: RollupProof) -> None:
    """
    Verify an inner rollup proof against its claimed public inputs.

    PRECOMPILE (production guest): recursive STARK verification.
        Same primitive as `verify_l2_execution_proof` (§rollup.py) — the zkVM's
        in-circuit recursive verifier. `program_vk` is the explicit verify-key
        parameter it checks against (§ProgramVK anchoring); the aggregation guest
        passes the same `program_vk` it bubbles up into `rollup_vks` /
        `program_vks`, so the anchored VK is provably the key the verification
        ran against. `RollupProof.proof` stands in for the recursive STARK bytes
        the guest would actually check. Beyond the recursive verify, we only
        re-validate the hash preimages (`l2L1BridgeTransactionTree`,
        `filteredAddressesHash`) the rollup-aggregation proof consumes.
    """
    # First: the recursive STARK verify against the explicit verify key.
    recursive_stark_verify(program_vk, proof.proof)
    # PRECOMPILE: keccak256 (preimage-binding checks).
    if hash_hash_list(proof.l2_l1_roots) != proof.public_inputs.l2_l1_bridge_transaction_tree:
        raise Exception("invalid l2L1BridgeTransactionTree preimage")
    if hash_address_list(proof.filtered_addresses) != proof.public_inputs.filtered_addresses_hash:
        raise Exception("invalid rollup filteredAddressesHash preimage")


def assert_rollup_proof_continuity(left: RollupProof, right: RollupProof) -> None:
    # Block-number / block-hash continuity is implicit in the shnarf check:
    # `endShnarf = Hash(parentShnarf, lastBlockHash, blobHash)` binds the
    # last block hash. Once the next blob's `parentShnarf` matches, the
    # inner block-hash chain inside that blob anchors block numbers
    # transitively. No separate block-number assertion is needed at this
    # layer (the rollup PI does not expose `startBlockNumber` anyway).
    if left.public_inputs.end_shnarf != right.public_inputs.parent_shnarf:
        raise Exception("rollup shnarf continuity failed")
    if left.public_inputs.end_l1_l2_bridge_rolling_hash != right.public_inputs.parent_l1_l2_bridge_rolling_hash:
        raise Exception("rollup L1-to-L2 rolling-hash continuity failed")
    if (
        left.public_inputs.end_l1_l2_bridge_rolling_hash_message_number !=
        right.public_inputs.parent_l1_l2_bridge_rolling_hash_message_number
    ):
        raise Exception("rollup L1-to-L2 rolling-hash-number continuity failed")
    if left.public_inputs.dynamic_chain_config_hash != right.public_inputs.dynamic_chain_config_hash:
        raise Exception("rollup dynamic chain configuration continuity failed")
    if left.public_inputs.end_ftx_rolling_hash != right.public_inputs.parent_ftx_rolling_hash:
        raise Exception("rollup FTX rolling-hash continuity failed")
    if left.public_inputs.end_processed_ftx_number != right.public_inputs.parent_processed_ftx_number:
        raise Exception("rollup processed-FTX-number continuity failed")
