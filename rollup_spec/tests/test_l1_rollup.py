"""
Business-oriented tests for L1 finalization ProgramVK anchoring
(`l1_rollup.finalize_rollup`).

These assert observable finalization behavior — whether a finalization is
accepted or reverted, and the resulting on-chain state — not internal wiring.
The §ProgramVK-anchoring rule under test: L1 keeps a single combined
`approved_vks` set, and `finalize_rollup` reverts any finalization whose
committed VKs (the one combined `public_inputs.program_vks` list, order bound to
the proof) are not all approved. Exec vs rollup is NOT distinguished on L1.

A `_base_state()` / `_base_submission()` pair is built so every pre-existing
finalization check passes trivially, isolating the VK-approval check as the only
variable across tests.

Run from the rollup_spec/ directory:  python -m pytest
"""

import pytest

from ethereum.crypto.hash import Hash32, keccak256
from ethereum_types.numeric import U64

from rollup_spec.l1_rollup import (
    FinalizationSubmission,
    LinethRollupState,
    PlonkVerifier,
    finalize_rollup,
    reinitialize_linea_rollup_v10,
    seed_genesis_position,
)
from rollup_spec.l2_execution import hash_address_list, hash_digest_list
from rollup_spec.rollup import RollupPublicInput

# Distinct VK / hash byte-pattern helpers (mirrors the style in
# test_proof_io_v1.py). Origins are noted only for tracing — L1 treats them as
# one combined list: 0xAA/0xA1 originate as exec VKs, 0xBB as a rollup VK.
_EXEC_VK_A = Hash32(bytes([0xAA]) * 32)
_EXEC_VK_B = Hash32(bytes([0xA1]) * 32)
_ROLLUP_VK = Hash32(bytes([0xBB]) * 32)

_PARENT_DATA_ROLLING_HASH = Hash32(bytes([0x47]) * 32)
_END_DATA_ROLLING_HASH = Hash32(bytes([0x8D]) * 32)
_END_OFFSET = 500
_PARENT_BLOCK_HASH = Hash32(bytes([0x46]) * 32)
_END_BLOCK_HASH = Hash32(bytes([0x9A]) * 32)
_L1L2_ROLLING_HASH = Hash32(bytes([0x22]) * 32)
_FTX_ROLLING_HASH = Hash32(bytes([0x44]) * 32)
_CHAIN_CONFIG_HASH = Hash32(bytes([0xC0]) * 32)
_PARENT_STATE_ROOT_HASH = Hash32(bytes([0x5E]) * 32)
_LEGACY_SHNARF = Hash32(bytes([0x71]) * 32)


def _base_state(approved_vks) -> LinethRollupState:
    """
    An L1 state whose continuity anchors exactly match `_base_submission()`'s
    public inputs, so all non-VK finalization checks pass. `approved_vks` is the
    only knob the tests vary.
    """
    return LinethRollupState(
        current_data_rolling_hash=_PARENT_DATA_ROLLING_HASH,
        current_data_availability_offset=0,
        current_l2_block_number=U64(1000500),
        block_hashes={U64(1000500): _PARENT_BLOCK_HASH},
        current_l2_block_timestamp=U64(1763000000),
        current_finalized_l1_l2_bridge_rolling_hash=_L1L2_ROLLING_HASH,
        current_finalized_l1_l2_bridge_rolling_hash_message_number=U64(0),
        current_finalized_ftx_rolling_hash=_FTX_ROLLING_HASH,
        current_finalized_processed_ftx_number=U64(7),
        verifier=PlonkVerifier(chain_configuration_hash=_CHAIN_CONFIG_HASH),
        anchored_data_rolling_hashes={_END_DATA_ROLLING_HASH},
        approved_vks=set(approved_vks),
    )


def _base_submission(program_vks) -> FinalizationSubmission:
    """
    A finalization submission carrying the single combined `program_vks` list
    nested in the PI (order bound to the proof). Empty `l2_l1_roots` /
    `filtered_addresses` keep the preimage-hash checks trivial (their keccak of
    empty input is the PI hash), and the FTX/rolling-hash boundary values are
    held constant across parent/end so continuity passes without any FTX deadline
    machinery. `start_offset=0` is the fresh-start case, which `finalize_rollup`
    accepts regardless of the previously-finalized offset.
    """
    pi = RollupPublicInput(
        end_block_number=U64(1000520),
        end_block_timestamp=U64(1763000457),
        l2_l1_bridge_transaction_tree=hash_digest_list([]),
        parent_l1_l2_bridge_rolling_hash=_L1L2_ROLLING_HASH,
        parent_l1_l2_bridge_rolling_hash_message_number=U64(0),
        end_l1_l2_bridge_rolling_hash=_L1L2_ROLLING_HASH,
        end_l1_l2_bridge_rolling_hash_message_number=U64(0),
        dynamic_chain_config_hash=_CHAIN_CONFIG_HASH,
        parent_ftx_rolling_hash=_FTX_ROLLING_HASH,
        parent_ftx_number=U64(7),
        end_ftx_rolling_hash=_FTX_ROLLING_HASH,
        end_processed_ftx_number=U64(7),
        filtered_addresses_hash=hash_address_list([]),
        parent_data_rolling_hash=_PARENT_DATA_ROLLING_HASH,
        end_data_rolling_hash=_END_DATA_ROLLING_HASH,
        parent_block_hash=_PARENT_BLOCK_HASH,
        end_block_hash=_END_BLOCK_HASH,
        start_offset=0,
        end_offset=_END_OFFSET,
        program_vks=list(program_vks),
    )
    return FinalizationSubmission(
        public_inputs=pi,
        proof=b"",
        l2_l1_roots=[],
        filtered_addresses=[],
        l2_messaging_blocks_offsets=[],
    )


def _finalize(state: LinethRollupState, submission: FinalizationSubmission) -> None:
    finalize_rollup(state, submission)


def test_finalize_rollup_rejects_unapproved_vk() -> None:
    # One committed VK (0xbb, rollup-origin) is NOT on the approved list — the
    # "operator swapped in an unapproved guest" case. L1 does not distinguish
    # exec vs rollup, so the single generic check rejects it.
    state = _base_state(approved_vks={_EXEC_VK_A})
    initial_data_rolling_hash = state.current_data_rolling_hash
    submission = _base_submission(program_vks=[_EXEC_VK_A, _ROLLUP_VK])
    with pytest.raises(Exception, match="program VK is not approved"):
        _finalize(state, submission)
    # Finalization reverted: state is unchanged.
    assert state.current_data_rolling_hash == initial_data_rolling_hash


def test_finalize_rollup_accepts_two_approved_exec_vks() -> None:
    # Goal-2b / multi-version finalization: a single finalization whose rollup
    # proofs carried TWO different (both approved) exec VKs — two forks
    # aggregated together, aggregation-grained — must succeed. The `program_vks`
    # PI is the canonical sorted-distinct set, so the input is sorted ascending
    # by byte value: 0xA1 (_EXEC_VK_B) < 0xAA (_EXEC_VK_A) < 0xBB (_ROLLUP_VK).
    state = _base_state(approved_vks={_EXEC_VK_A, _EXEC_VK_B, _ROLLUP_VK})
    submission = _base_submission(program_vks=[_EXEC_VK_B, _EXEC_VK_A, _ROLLUP_VK])
    _finalize(state, submission)  # must not raise
    # Finalization applied: DA and execution continuity advanced to the
    # submission's end-of-range values.
    assert state.current_data_rolling_hash == _END_DATA_ROLLING_HASH
    assert state.current_data_availability_offset == _END_OFFSET
    assert state.block_hashes[U64(1000520)] == _END_BLOCK_HASH
    assert int(state.current_l2_block_number) == 1000520


def test_finalize_rollup_succeeds_when_all_vks_approved() -> None:
    # No-regression: the ordinary single-exec-VK + rollup-VK happy path still
    # finalizes when every committed VK is approved.
    state = _base_state(approved_vks={_EXEC_VK_A, _ROLLUP_VK})
    submission = _base_submission(program_vks=[_EXEC_VK_A, _ROLLUP_VK])
    _finalize(state, submission)  # must not raise
    assert state.current_data_rolling_hash == _END_DATA_ROLLING_HASH
    assert state.current_data_availability_offset == _END_OFFSET
    assert state.block_hashes[U64(1000520)] == _END_BLOCK_HASH
    assert int(state.current_l2_block_number) == 1000520


def test_finalize_rollup_rejects_nonmatching_start_offset() -> None:
    state = _base_state(approved_vks={_EXEC_VK_A, _ROLLUP_VK})
    state.current_data_availability_offset = 123
    with pytest.raises(Exception, match="startOffset"):
        _finalize(state, _base_submission(program_vks=[_EXEC_VK_A, _ROLLUP_VK]))


def test_finalize_rollup_uses_state_root_on_missing_parent_block_hash() -> None:
    # State-root → block-hash migration path: `block_hashes` has no entry for the last
    # finalized block, so finalization requires `parent_block_hash == EMPTY_HASH` and
    # instead matches the legacy `state_root_hashes` value via `parent_state_root_hash`.
    state = _base_state(approved_vks={_EXEC_VK_A, _ROLLUP_VK})
    state.block_hashes.clear()
    state.state_root_hashes[U64(1000500)] = _PARENT_STATE_ROOT_HASH
    submission = _base_submission(program_vks=[_EXEC_VK_A, _ROLLUP_VK])
    submission.public_inputs.parent_block_hash = Hash32(b"\x00" * 32)
    submission.parent_state_root_hash = _PARENT_STATE_ROOT_HASH
    _finalize(state, submission)
    # The migration anchors the new block hash, moving the next round onto the new path.
    assert state.block_hashes[U64(1000520)] == _END_BLOCK_HASH


def test_finalize_rollup_migration_rejects_wrong_state_root() -> None:
    # Migration path with a `parent_state_root_hash` that does not match the stored legacy
    # state root must revert (`StartingRootHashDoesNotMatch` on-chain).
    state = _base_state(approved_vks={_EXEC_VK_A, _ROLLUP_VK})
    state.block_hashes.clear()
    state.state_root_hashes[U64(1000500)] = _PARENT_STATE_ROOT_HASH
    submission = _base_submission(program_vks=[_EXEC_VK_A, _ROLLUP_VK])
    submission.public_inputs.parent_block_hash = Hash32(b"\x00" * 32)
    submission.parent_state_root_hash = Hash32(bytes([0xFF]) * 32)
    with pytest.raises(Exception, match="parentStateRootHash"):
        _finalize(state, submission)


def test_finalize_rollup_migration_rejects_nonempty_parent_block_hash() -> None:
    # Migration path requires `parent_block_hash == EMPTY_HASH`; a non-zero value must revert
    # (`StartingBlockHashDoesNotMatch` on-chain).
    state = _base_state(approved_vks={_EXEC_VK_A, _ROLLUP_VK})
    state.block_hashes.clear()
    state.state_root_hashes[U64(1000500)] = _PARENT_STATE_ROOT_HASH
    submission = _base_submission(program_vks=[_EXEC_VK_A, _ROLLUP_VK])
    submission.parent_state_root_hash = _PARENT_STATE_ROOT_HASH
    with pytest.raises(Exception, match="EMPTY_HASH on the migration path"):
        _finalize(state, submission)


def test_seed_genesis_position_anchors_genesis() -> None:
    # Fresh-network genesis: the dataRollingHash is deterministically seeded from the genesis
    # block hash (`keccak256(EMPTY_HASH || initialBlockHash)`), anchored, and the block hash
    # is stored. Offset stays 0 and there is no legacy shnarf to migrate.
    state = _base_state(approved_vks=set())
    state.current_data_rolling_hash = Hash32(b"\x00" * 32)
    state.block_hashes.clear()
    genesis_hash = seed_genesis_position(state, _PARENT_BLOCK_HASH, U64(1000500))
    assert genesis_hash == keccak256(Hash32(b"\x00" * 32) + _PARENT_BLOCK_HASH)
    assert state.current_data_rolling_hash == genesis_hash
    assert state.current_data_availability_offset == 0
    assert genesis_hash in state.anchored_data_rolling_hashes
    assert state.block_hashes[U64(1000500)] == _PARENT_BLOCK_HASH
    assert state.current_finalized_shnarf_deprecated == Hash32(b"\x00" * 32)


def test_reinitialize_linea_rollup_v10_migrates_legacy_shnarf() -> None:
    # In-place upgrade: the legacy `currentFinalizedShnarf` slot is reinterpreted as the live
    # currentDataRollingHash, anchored so post-upgrade submissions chain from it, and the
    # legacy slot is wiped. The next finalization then chains from the migrated value.
    state = _base_state(approved_vks={_EXEC_VK_A, _ROLLUP_VK})
    state.current_finalized_shnarf_deprecated = _LEGACY_SHNARF
    migrated = reinitialize_linea_rollup_v10(state)
    assert migrated == _LEGACY_SHNARF
    assert state.current_data_rolling_hash == _LEGACY_SHNARF
    assert _LEGACY_SHNARF in state.anchored_data_rolling_hashes
    assert state.current_finalized_shnarf_deprecated == Hash32(b"\x00" * 32)
