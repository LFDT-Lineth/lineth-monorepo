"""
Business-oriented tests for L1 finalization program-ID approval
(`l1_rollup.finalize_rollup`).

These assert observable finalization behavior — whether a finalization is
accepted or reverted, and the resulting on-chain state — not internal wiring.
The program-ID approval rule under test: L1 keeps a single combined
`approved_program_ids` set, and `finalize_rollup` reverts any finalization whose
committed IDs (the one combined `public_inputs.program_ids` list, order bound to
the proof) are not all approved.

A `_base_state()` / `_base_submission()` pair is built so every pre-existing
finalization check passes trivially, isolating the ID-approval check as the only
variable across tests.

Run from the rollup_spec/ directory:  python -m pytest
"""

import pytest
from ethereum.crypto.hash import Hash32, keccak256
from ethereum_types.numeric import U64

from rollup_spec.l1_rollup import (
    FinalizationPublicInput,
    FinalizationSubmission,
    LinethRollupState,
    PlonkVerifier,
    finalize_rollup,
)


# Distinct guest program IDs; L1 checks them in one combined list.
_EXEC_ID_A = Hash32(bytes([0xAA]) * 32)
_EXEC_ID_B = Hash32(bytes([0xA1]) * 32)
_ROLLUP_ID = Hash32(bytes([0xBB]) * 32)

_PARENT_DATA_ROLLING_HASH = Hash32(bytes([0x47]) * 32)
_END_DATA_ROLLING_HASH = Hash32(bytes([0x8D]) * 32)
_FINAL_DATA_TAIL_DISCARD = 500
_PARENT_BLOCK_HASH = Hash32(bytes([0x46]) * 32)
_END_BLOCK_HASH = Hash32(bytes([0x9A]) * 32)
_L1L2_ROLLING_HASH = Hash32(bytes([0x22]) * 32)
_FTX_ROLLING_HASH = Hash32(bytes([0x44]) * 32)
_CHAIN_CONFIG_HASH = Hash32(bytes([0xC0]) * 32)


def _position_commitment(data_rolling_hash: Hash32, tail_count: int) -> Hash32:
    """The `current_finalized_position_commitment` value sealing a given
    (dataRollingHash, tail count) end position (§3.6)."""
    return keccak256(data_rolling_hash + tail_count.to_bytes(32, "big"))


def _base_state(approved_program_ids, parent_data_tail_take: int = 0) -> LinethRollupState:
    """
    An L1 state whose continuity anchors exactly match `_base_submission()`'s
    public inputs, so all other finalization checks pass. `approved_program_ids` is the
    only knob the tests vary.
    """
    return LinethRollupState(
        current_finalized_position_commitment=_position_commitment(_PARENT_DATA_ROLLING_HASH, parent_data_tail_take),
        current_finalized_last_block_hash=_PARENT_BLOCK_HASH,
        current_l2_block_number=U64(1000500),
        current_l2_block_timestamp=U64(1763000000),
        current_finalized_l1_l2_bridge_rolling_hash=_L1L2_ROLLING_HASH,
        current_finalized_l1_l2_bridge_rolling_hash_message_number=U64(0),
        current_finalized_ftx_rolling_hash=_FTX_ROLLING_HASH,
        current_finalized_processed_ftx_number=U64(7),
        verifier=PlonkVerifier(chain_configuration_hash=_CHAIN_CONFIG_HASH),
        anchored_data_rolling_hashes={_END_DATA_ROLLING_HASH},
        approved_program_ids=set(approved_program_ids),
    )


def _base_submission(program_ids, parent_data_tail_take: int = 0) -> FinalizationSubmission:
    """
    A finalization submission carrying the single combined `program_ids` list
    nested in the PI (order bound to the proof). Empty `l2_l1_roots` /
    `filtered_addresses` are committed directly with the FTX/rolling-hash boundary values
    held constant across parent/end so continuity passes without any FTX deadline
    machinery.
    """
    pi = FinalizationPublicInput(
        end_block_number=U64(1000520),
        end_block_timestamp=U64(1763000457),
        parent_l1_l2_bridge_rolling_hash=_L1L2_ROLLING_HASH,
        parent_l1_l2_bridge_rolling_hash_message_number=U64(0),
        end_l1_l2_bridge_rolling_hash=_L1L2_ROLLING_HASH,
        end_l1_l2_bridge_rolling_hash_message_number=U64(0),
        dynamic_chain_config_hash=_CHAIN_CONFIG_HASH,
        parent_ftx_rolling_hash=_FTX_ROLLING_HASH,
        parent_ftx_number=U64(7),
        end_ftx_rolling_hash=_FTX_ROLLING_HASH,
        end_processed_ftx_number=U64(7),
        parent_data_rolling_hash=_PARENT_DATA_ROLLING_HASH,
        end_data_rolling_hash=_END_DATA_ROLLING_HASH,
        parent_block_hash=_PARENT_BLOCK_HASH,
        end_block_hash=_END_BLOCK_HASH,
        parent_data_tail_take=parent_data_tail_take,
        final_data_tail_discard=_FINAL_DATA_TAIL_DISCARD,
        l2_l1_tree_depth=5,
        l2_l1_roots=[],
        filtered_addresses=[],
        program_ids=list(program_ids),
    )
    return FinalizationSubmission(
        public_inputs=pi,
        proof=b"",
    )


def test_finalization_emits_exact_proven_messaging_offsets() -> None:
    state = _base_state(approved_program_ids=set())
    submission = _base_submission(program_ids=[])
    submission.public_inputs.l2_messaging_blocks_offsets = [1, 7, 20]
    assert finalize_rollup(state, submission, _PARENT_DATA_ROLLING_HASH, 0) == b"\x00\x01\x00\x07\x00\x14"


def test_finalization_rejects_out_of_range_or_repeated_messaging_offsets() -> None:
    for offsets in ([0], [21], [7, 7], [65536]):
        state = _base_state(approved_program_ids=set())
        submission = _base_submission(program_ids=[])
        submission.public_inputs.l2_messaging_blocks_offsets = offsets
        with pytest.raises(Exception, match="messaging block offset"):
            finalize_rollup(state, submission, _PARENT_DATA_ROLLING_HASH, 0)


def _finalize(state: LinethRollupState, submission: FinalizationSubmission) -> None:
    """`finalize_rollup`, supplying the (dataRollingHash, tail count) pair that opens
    `_base_state()`'s position commitment."""
    finalize_rollup(state, submission, _PARENT_DATA_ROLLING_HASH, 0)


def test_finalize_rollup_rejects_unapproved_program_id() -> None:
    # A finalization containing an unapproved guest program ID is rejected.
    state = _base_state(approved_program_ids={_EXEC_ID_A})
    initial_commitment = state.current_finalized_position_commitment
    submission = _base_submission(program_ids=[_EXEC_ID_A, _ROLLUP_ID])
    with pytest.raises(Exception, match="program ID is not approved"):
        _finalize(state, submission)
    # Finalization reverted: state is unchanged.
    assert state.current_finalized_position_commitment == initial_commitment


def test_finalize_rollup_accepts_multiple_approved_program_ids() -> None:
    # Multiple approved guest programs can finalize together. The IDs are sorted
    # by byte value: 0xA1 (_EXEC_ID_B) < 0xAA (_EXEC_ID_A) < 0xBB (_ROLLUP_ID).
    state = _base_state(approved_program_ids={_EXEC_ID_A, _EXEC_ID_B, _ROLLUP_ID})
    submission = _base_submission(program_ids=[_EXEC_ID_B, _EXEC_ID_A, _ROLLUP_ID])
    _finalize(state, submission)  # must not raise
    # Finalization applied: block hash, block number, and position commitment
    # all advanced to the submission's end-of-range values.
    assert state.current_finalized_last_block_hash == _END_BLOCK_HASH
    assert int(state.current_l2_block_number) == 1000520
    assert state.current_finalized_position_commitment == _position_commitment(_END_DATA_ROLLING_HASH, _FINAL_DATA_TAIL_DISCARD)


def test_finalize_rollup_succeeds_when_all_program_ids_approved() -> None:
    # A finalization succeeds when every committed ID is approved.
    state = _base_state(approved_program_ids={_EXEC_ID_A, _ROLLUP_ID})
    submission = _base_submission(program_ids=[_EXEC_ID_A, _ROLLUP_ID])
    _finalize(state, submission)  # must not raise
    assert state.current_finalized_position_commitment == _position_commitment(_END_DATA_ROLLING_HASH, _FINAL_DATA_TAIL_DISCARD)
    assert int(state.current_l2_block_number) == 1000520


def test_finalize_rollup_rejects_chunk_boundary_start_inside_finalized_blob() -> None:
    parent_data_tail_take = 9
    state = _base_state(approved_program_ids=set(), parent_data_tail_take=parent_data_tail_take)
    submission = _base_submission(program_ids=[], parent_data_tail_take=0)

    with pytest.raises(Exception, match="parentDataTailTake does not match the finalized position"):
        finalize_rollup(state, submission, _PARENT_DATA_ROLLING_HASH, parent_data_tail_take)
