"""
Business-oriented tests for L1 finalization (`l1_rollup.finalize_rollup`).

These assert observable finalization behavior — whether a finalization is
accepted or reverted, and the resulting on-chain state — not internal wiring.
The approval rule under test: L1 keeps a single combined `approved_vks` set
(`verifierKeys` on-chain), and `finalize_rollup` reverts any finalization whose
committed IDs (the one combined `public_inputs.program_ids` list, order bound to
the proof) are not all approved. The DA boundary is the plain
`(current_data_rolling_hash, current_data_tail_bytes)` pair and must match the
PI's `(parent_data_rolling_hash, parent_data_tail_take_bytes)` exactly.

A `_base_state()` / `_base_submission()` pair is built so every pre-existing
finalization check passes trivially, isolating the check under test as the only
variable across tests.

Run from the rollup_spec/ directory:  python -m pytest
"""

import pytest
from ethereum.crypto.hash import Hash32, keccak256
from ethereum_types.numeric import U64

from rollup_spec.l1_rollup import (
    MAX_DATA_TAIL_BYTES,
    FinalizationPublicInput,
    FinalizationSubmission,
    LinethRollupState,
    PlonkVerifier,
    compute_public_input,
    finalize_rollup,
    reinitialize_linea_rollup_v10,
    seed_genesis_position,
)
from rollup_spec.l2_execution import ZERO_HASH


# Distinct guest program IDs; L1 checks them in one combined list.
_EXEC_ID_A = Hash32(bytes([0xAA]) * 32)
_EXEC_ID_B = Hash32(bytes([0xA1]) * 32)
_ROLLUP_ID = Hash32(bytes([0xBB]) * 32)

_PARENT_DATA_ROLLING_HASH = Hash32(bytes([0x47]) * 32)
_END_DATA_ROLLING_HASH = Hash32(bytes([0x8D]) * 32)
_FINAL_DATA_TAIL_DISCARD = 500
_PARENT_BLOCK_HASH = Hash32(bytes([0x46]) * 32)
_END_BLOCK_HASH = Hash32(bytes([0x9A]) * 32)
_FTX_ROLLING_HASH = Hash32(bytes([0x44]) * 32)
_CHAIN_CONFIG_HASH = Hash32(bytes([0xC0]) * 32)
_LEGACY_SHNARF = Hash32(bytes([0x71]) * 32)
_ZERO_HASH = Hash32(b"\x00" * 32)


def _base_state(approved_vks, current_data_tail_bytes: int = 0) -> LinethRollupState:
    """
    An L1 state whose continuity anchors exactly match `_base_submission()`'s
    public inputs, so all other finalization checks pass. `approved_vks` is the
    main knob the tests vary.
    """
    return LinethRollupState(
        current_data_rolling_hash=_PARENT_DATA_ROLLING_HASH,
        current_data_tail_bytes=current_data_tail_bytes,
        current_l2_block_number=U64(1000500),
        block_hashes={U64(1000500): _PARENT_BLOCK_HASH},
        current_l2_block_timestamp=U64(1763000000),
        current_finalized_l1_l2_bridge_rolling_hash=ZERO_HASH,
        current_finalized_l1_l2_bridge_rolling_hash_message_number=U64(0),
        current_finalized_ftx_rolling_hash=_FTX_ROLLING_HASH,
        current_finalized_processed_ftx_number=U64(7),
        verifier=PlonkVerifier(chain_configuration_hash=_CHAIN_CONFIG_HASH),
        anchored_data_rolling_hashes={_END_DATA_ROLLING_HASH},
        approved_vks=set(approved_vks),
    )


def _base_submission(program_ids, parent_data_tail_take_bytes: int = 0) -> FinalizationSubmission:
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
        end_l1_l2_bridge_rolling_hash=ZERO_HASH,
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
        parent_data_tail_take_bytes=parent_data_tail_take_bytes,
        final_data_tail_discard_bytes=_FINAL_DATA_TAIL_DISCARD,
        l2_l1_tree_depth=5,
        l2_l1_roots=[],
        filtered_addresses=[],
        program_ids=list(program_ids),
    )
    return FinalizationSubmission(
        public_inputs=pi,
        proof=b"",
    )


def _finalize(state: LinethRollupState, submission: FinalizationSubmission) -> None:
    finalize_rollup(state, submission)


def test_finalization_emits_exact_proven_messaging_offsets() -> None:
    state = _base_state(approved_vks=set())
    submission = _base_submission(program_ids=[])
    submission.public_inputs.l2_messaging_blocks_offsets = [1, 7, 20]
    assert finalize_rollup(state, submission) == b"\x00\x01\x00\x07\x00\x14"


def test_finalize_rollup_rejects_unapproved_program_id() -> None:
    # One committed ID (0xbb, rollup-origin) is NOT on the approved list — the
    # "operator swapped in an unapproved guest" case. L1 does not distinguish
    # exec vs rollup, so the single generic check rejects it.
    state = _base_state(approved_vks={_EXEC_ID_A})
    initial_data_rolling_hash = state.current_data_rolling_hash
    initial_tail_bytes = state.current_data_tail_bytes
    submission = _base_submission(program_ids=[_EXEC_ID_A, _ROLLUP_ID])
    with pytest.raises(Exception, match="program ID is not an approved verifier key"):
        _finalize(state, submission)
    # Finalization reverted: state is unchanged.
    assert state.current_data_rolling_hash == initial_data_rolling_hash
    assert state.current_data_tail_bytes == initial_tail_bytes


def test_finalize_rollup_accepts_multiple_approved_program_ids() -> None:
    # Multiple approved guest programs can finalize together. The IDs are sorted
    # by byte value: 0xA1 (_EXEC_ID_B) < 0xAA (_EXEC_ID_A) < 0xBB (_ROLLUP_ID).
    state = _base_state(approved_vks={_EXEC_ID_A, _EXEC_ID_B, _ROLLUP_ID})
    submission = _base_submission(program_ids=[_EXEC_ID_B, _EXEC_ID_A, _ROLLUP_ID])
    _finalize(state, submission)  # must not raise
    # Finalization applied: DA and execution continuity advanced to the
    # submission's end-of-range values.
    assert state.current_data_rolling_hash == _END_DATA_ROLLING_HASH
    assert state.current_data_tail_bytes == _FINAL_DATA_TAIL_DISCARD
    assert state.block_hashes[U64(1000520)] == _END_BLOCK_HASH
    assert int(state.current_l2_block_number) == 1000520


def test_finalize_rollup_succeeds_when_all_program_ids_approved() -> None:
    state = _base_state(approved_vks={_EXEC_ID_A, _ROLLUP_ID})
    submission = _base_submission(program_ids=[_EXEC_ID_A, _ROLLUP_ID])
    _finalize(state, submission)  # must not raise
    assert state.current_data_rolling_hash == _END_DATA_ROLLING_HASH
    assert state.current_data_tail_bytes == _FINAL_DATA_TAIL_DISCARD
    assert state.block_hashes[U64(1000520)] == _END_BLOCK_HASH
    assert int(state.current_l2_block_number) == 1000520


def test_finalize_rollup_rejects_chunk_boundary_start_inside_finalized_blob() -> None:
    # The stored boundary is mid-blob (9 tail bytes pending) but the PI claims a chunk-boundary
    # start (take 0): the pair must match exactly, so this reverts.
    state = _base_state(approved_vks=set(), current_data_tail_bytes=9)
    submission = _base_submission(program_ids=[], parent_data_tail_take_bytes=0)
    with pytest.raises(Exception, match="parentDataTailTakeBytes does not match"):
        _finalize(state, submission)


def test_finalize_rollup_rejects_nonmatching_parent_data_rolling_hash() -> None:
    state = _base_state(approved_vks=set())
    state.current_data_rolling_hash = Hash32(bytes([0x01]) * 32)
    with pytest.raises(Exception, match="parentDataRollingHash"):
        _finalize(state, _base_submission(program_ids=[]))


def test_finalize_rollup_migration_path_accepts_empty_parent_block_hash() -> None:
    # State-root → block-hash migration path: `block_hashes` has no entry for the last
    # finalized block (the parent was committed under the old state-root model), so
    # finalization requires `parent_block_hash == EMPTY_HASH` and proceeds on that signal
    # alone — there is no parent block hash to soft-check against.
    state = _base_state(approved_vks={_EXEC_ID_A, _ROLLUP_ID})
    state.block_hashes.clear()
    submission = _base_submission(program_ids=[_EXEC_ID_A, _ROLLUP_ID])
    submission.public_inputs.parent_block_hash = _ZERO_HASH
    _finalize(state, submission)
    # The migration anchors the new block hash, moving the next round onto the new path.
    assert state.block_hashes[U64(1000520)] == _END_BLOCK_HASH


def test_finalize_rollup_migration_rejects_nonempty_parent_block_hash() -> None:
    # Migration path requires `parent_block_hash == EMPTY_HASH`; a non-zero value must revert
    # (`StartingBlockHashDoesNotMatch` on-chain).
    state = _base_state(approved_vks={_EXEC_ID_A, _ROLLUP_ID})
    state.block_hashes.clear()
    submission = _base_submission(program_ids=[_EXEC_ID_A, _ROLLUP_ID])
    with pytest.raises(Exception, match="EMPTY_HASH on the migration path"):
        _finalize(state, submission)


def test_finalize_rollup_rejects_out_of_range_final_tail_discard() -> None:
    # `final_data_tail_discard_bytes` beyond a single blob chunk (`MAX_DATA_TAIL_BYTES`) must
    # revert (`DataTailBytesOutOfRange` on-chain). The parent tail stays continuous (= 0) so only
    # the final-tail bound is the failing check.
    state = _base_state(approved_vks=set())
    submission = _base_submission(program_ids=[])
    submission.public_inputs.final_data_tail_discard_bytes = MAX_DATA_TAIL_BYTES + 1
    with pytest.raises(Exception, match="finalDataTailDiscardBytes out of range"):
        _finalize(state, submission)


def test_finalize_rollup_accepts_max_tail_boundary() -> None:
    # The inclusive upper bound `final_data_tail_discard_bytes == MAX_DATA_TAIL_BYTES` is accepted.
    state = _base_state(approved_vks=set())
    submission = _base_submission(program_ids=[])
    submission.public_inputs.final_data_tail_discard_bytes = MAX_DATA_TAIL_BYTES
    _finalize(state, submission)  # must not raise
    assert state.current_data_tail_bytes == MAX_DATA_TAIL_BYTES


def test_finalize_rollup_rejects_out_of_range_parent_tail_take() -> None:
    # `parent_data_tail_take_bytes` beyond `MAX_DATA_TAIL_BYTES` must revert even if it happened
    # to match the stored `current_data_tail_bytes` (here both are forced past the bound).
    state = _base_state(approved_vks=set(), current_data_tail_bytes=MAX_DATA_TAIL_BYTES + 1)
    submission = _base_submission(program_ids=[], parent_data_tail_take_bytes=MAX_DATA_TAIL_BYTES + 1)
    with pytest.raises(Exception, match="parentDataTailTakeBytes out of range"):
        _finalize(state, submission)


def test_finalize_rollup_same_data_rolling_hash_requires_tail_progress() -> None:
    # Same-dataRollingHash guard: no new chunk is anchored, so the finalization only advances
    # inside an already-shared chunk. Take (100) must exceed discard (500) to make progress.
    state = _base_state(approved_vks=set(), current_data_tail_bytes=100)
    state.current_data_rolling_hash = _END_DATA_ROLLING_HASH
    submission = _base_submission(program_ids=[], parent_data_tail_take_bytes=100)
    submission.public_inputs.parent_data_rolling_hash = _END_DATA_ROLLING_HASH
    with pytest.raises(Exception, match="invalid data tail progression"):
        _finalize(state, submission)


def test_finalize_rollup_same_data_rolling_hash_accepts_tail_progress() -> None:
    state = _base_state(approved_vks=set(), current_data_tail_bytes=800)
    state.current_data_rolling_hash = _END_DATA_ROLLING_HASH
    submission = _base_submission(program_ids=[], parent_data_tail_take_bytes=800)
    submission.public_inputs.parent_data_rolling_hash = _END_DATA_ROLLING_HASH
    _finalize(state, submission)  # must not raise
    assert state.current_data_tail_bytes == _FINAL_DATA_TAIL_DISCARD


def test_seed_genesis_position_anchors_genesis() -> None:
    # Fresh-network genesis: the dataRollingHash is deterministically seeded from the genesis
    # block hash (`keccak256(EMPTY_HASH || initialBlockHash)`), anchored, and the block hash
    # is stored. Tail bytes stay 0 and there is no legacy shnarf to migrate.
    state = _base_state(approved_vks=set())
    state.current_data_rolling_hash = _ZERO_HASH
    state.block_hashes.clear()
    genesis_hash = seed_genesis_position(state, _PARENT_BLOCK_HASH, U64(1000500))
    assert genesis_hash == keccak256(_ZERO_HASH + _PARENT_BLOCK_HASH)
    assert state.current_data_rolling_hash == genesis_hash
    assert state.current_data_tail_bytes == 0
    assert genesis_hash in state.anchored_data_rolling_hashes
    assert state.block_hashes[U64(1000500)] == _PARENT_BLOCK_HASH
    assert state.current_finalized_shnarf_deprecated == _ZERO_HASH


def test_reinitialize_linea_rollup_v10_migrates_legacy_shnarf() -> None:
    # In-place upgrade: the legacy `currentFinalizedShnarf` slot is reinterpreted as the live
    # currentDataRollingHash, anchored so post-upgrade submissions chain from it, and the
    # legacy slot is wiped. The next finalization then chains from the migrated value.
    state = _base_state(approved_vks={_EXEC_ID_A, _ROLLUP_ID})
    state.current_finalized_shnarf_deprecated = _LEGACY_SHNARF
    migrated = reinitialize_linea_rollup_v10(state)
    assert migrated == _LEGACY_SHNARF
    assert state.current_data_rolling_hash == _LEGACY_SHNARF
    assert _LEGACY_SHNARF in state.anchored_data_rolling_hashes
    assert state.current_finalized_shnarf_deprecated == _ZERO_HASH


def test_reinitialize_linea_rollup_v10_rejects_zero_legacy_shnarf() -> None:
    # Mirrors the contract's `ZeroHashNotAllowed` guard in `_migrateLegacyShnarf`: a proxy that
    # never held a legacy shnarf must not anchor the zero hash as a valid parent dataRollingHash.
    state = _base_state(approved_vks={_EXEC_ID_A, _ROLLUP_ID})
    state.current_finalized_shnarf_deprecated = _ZERO_HASH
    with pytest.raises(Exception, match="ZeroHashNotAllowed"):
        reinitialize_linea_rollup_v10(state)
    assert _ZERO_HASH not in state.anchored_data_rolling_hashes


def test_compute_public_input_binds_l2_messaging_blocks_offsets() -> None:
    # `l2MessagingBlocksOffsets` is hashed into the on-chain public input like the other
    # preimages: tampering with the offsets changes the computed `public_input`, so a proof
    # attesting to the original would no longer verify.
    state = _base_state(approved_vks={_EXEC_ID_A, _ROLLUP_ID})
    original = _base_submission(program_ids=[_EXEC_ID_A, _ROLLUP_ID])
    tampered = _base_submission(program_ids=[_EXEC_ID_A, _ROLLUP_ID])
    tampered.public_inputs.l2_messaging_blocks_offsets = [7, 9]
    assert compute_public_input(original, state) != compute_public_input(tampered, state)
    # Determinism / bound range: the public input is a BN254 scalar-field element.
    assert compute_public_input(original, state) == compute_public_input(original, state)
    assert 0 <= compute_public_input(original, state) < (1 << 254)


def test_compute_public_input_binds_data_tail_bytes() -> None:
    # The tail byte counts are part of the packed public input (replacing the old offsets).
    state = _base_state(approved_vks=set())
    original = _base_submission(program_ids=[])
    tampered = _base_submission(program_ids=[], parent_data_tail_take_bytes=9)
    assert compute_public_input(original, state) != compute_public_input(tampered, state)


_FINALIZED_ROLLING_HASH = Hash32(bytes([0x22]) * 32)
_ANCHORED_ROLLING_HASH = Hash32(bytes([0x55]) * 32)


def _state_with_l1_l2_rolling_hashes() -> LinethRollupState:
    """A state that finalized L1->L2 message 3 and holds L1's authoritative rolling hashes for messages 3 and 5."""
    state = _base_state(approved_vks=set())
    state.current_finalized_l1_l2_bridge_rolling_hash = _FINALIZED_ROLLING_HASH
    state.current_finalized_l1_l2_bridge_rolling_hash_message_number = U64(3)
    state.l1_l2_rolling_hashes = {U64(3): _FINALIZED_ROLLING_HASH, U64(5): _ANCHORED_ROLLING_HASH}
    return state


def _submission_ending_at(rolling_hash: Hash32, message_number: int) -> FinalizationSubmission:
    submission = _base_submission(program_ids=[])
    submission.public_inputs.end_l1_l2_bridge_rolling_hash = rolling_hash
    submission.public_inputs.end_l1_l2_bridge_rolling_hash_message_number = U64(message_number)
    return submission


def test_finalization_that_anchored_no_messages_keeps_the_stored_rolling_hash_pair() -> None:
    state = _state_with_l1_l2_rolling_hashes()

    _finalize(state, _submission_ending_at(ZERO_HASH, 0))

    assert state.current_finalized_l1_l2_bridge_rolling_hash == _FINALIZED_ROLLING_HASH
    assert int(state.current_finalized_l1_l2_bridge_rolling_hash_message_number) == 3


def test_finalization_with_an_authentic_advancing_rolling_hash_updates_the_stored_pair() -> None:
    state = _state_with_l1_l2_rolling_hashes()

    _finalize(state, _submission_ending_at(_ANCHORED_ROLLING_HASH, 5))

    assert state.current_finalized_l1_l2_bridge_rolling_hash == _ANCHORED_ROLLING_HASH
    assert int(state.current_finalized_l1_l2_bridge_rolling_hash_message_number) == 5


def test_finalization_rejects_a_rolling_hash_without_a_message_number() -> None:
    state = _state_with_l1_l2_rolling_hashes()

    with pytest.raises(Exception, match="must be zero"):
        _finalize(state, _submission_ending_at(_ANCHORED_ROLLING_HASH, 0))

    assert state.current_finalized_l1_l2_bridge_rolling_hash == _FINALIZED_ROLLING_HASH


def test_finalization_rejects_a_rolling_hash_that_differs_from_l1_storage() -> None:
    state = _state_with_l1_l2_rolling_hashes()

    with pytest.raises(Exception, match="does not match L1 storage"):
        _finalize(state, _submission_ending_at(Hash32(bytes([0x66]) * 32), 5))

    assert int(state.current_finalized_l1_l2_bridge_rolling_hash_message_number) == 3


def test_finalization_rejects_a_message_number_l1_never_anchored() -> None:
    state = _state_with_l1_l2_rolling_hashes()

    with pytest.raises(Exception, match="does not match L1 storage"):
        _finalize(state, _submission_ending_at(_ANCHORED_ROLLING_HASH, 6))


def test_finalization_rejects_a_message_number_below_the_stored_one() -> None:
    state = _state_with_l1_l2_rolling_hashes()
    state.l1_l2_rolling_hashes[U64(2)] = Hash32(bytes([0x11]) * 32)
    submission = _submission_ending_at(state.l1_l2_rolling_hashes[U64(2)], 2)

    with pytest.raises(Exception, match="must not decrease"):
        _finalize(state, submission)

    assert int(state.current_finalized_l1_l2_bridge_rolling_hash_message_number) == 3


def test_finalization_accepts_the_stored_pair_carried_forward_explicitly() -> None:
    # Equal is fine: the prover re-emits the parent aggregation's pair when no new L1 message
    # was anchored in the range, and the stored pair is left unchanged.
    state = _state_with_l1_l2_rolling_hashes()

    _finalize(state, _submission_ending_at(_FINALIZED_ROLLING_HASH, 3))

    assert state.current_finalized_l1_l2_bridge_rolling_hash == _FINALIZED_ROLLING_HASH
    assert int(state.current_finalized_l1_l2_bridge_rolling_hash_message_number) == 3
