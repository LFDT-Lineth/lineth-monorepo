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

from ethereum.crypto.hash import Hash32
from ethereum_types.numeric import U64

from rollup_spec.l1_rollup import (
    FinalizationSubmission,
    LinethRollupState,
    PlonkVerifier,
    finalize_rollup,
)
from rollup_spec.l2_execution import hash_address_list, hash_digest_list
from rollup_spec.rollup import RollupPublicInput

# Distinct VK / hash byte-pattern helpers (mirrors the style in
# test_proof_io_v1.py). Origins are noted only for tracing — L1 treats them as
# one combined list: 0xAA/0xA1 originate as exec VKs, 0xBB as a rollup VK.
_EXEC_VK_A = Hash32(bytes([0xAA]) * 32)
_EXEC_VK_B = Hash32(bytes([0xA1]) * 32)
_ROLLUP_VK = Hash32(bytes([0xBB]) * 32)

_PARENT_SHNARF = Hash32(bytes([0x47]) * 32)
_END_SHNARF = Hash32(bytes([0x8D]) * 32)
_END_LAST_BLOCK_HASH = Hash32(bytes([0x9A]) * 32)
_L1L2_ROLLING_HASH = Hash32(bytes([0x22]) * 32)
_FTX_ROLLING_HASH = Hash32(bytes([0x44]) * 32)
_CHAIN_CONFIG_HASH = Hash32(bytes([0xC0]) * 32)


def _base_state(approved_vks) -> LinethRollupState:
    """
    An L1 state whose continuity anchors exactly match `_base_submission()`'s
    public inputs, so all non-VK finalization checks pass. `approved_vks` is the
    only knob the tests vary.
    """
    return LinethRollupState(
        current_finalized_shnarf=_PARENT_SHNARF,
        current_finalized_last_block_hash=Hash32(bytes([0x46]) * 32),
        current_l2_block_number=U64(1000500),
        current_l2_block_timestamp=U64(1763000000),
        current_finalized_l1_l2_bridge_rolling_hash=_L1L2_ROLLING_HASH,
        current_finalized_l1_l2_bridge_rolling_hash_message_number=U64(0),
        current_finalized_ftx_rolling_hash=_FTX_ROLLING_HASH,
        current_finalized_processed_ftx_number=U64(7),
        verifier=PlonkVerifier(chain_configuration_hash=_CHAIN_CONFIG_HASH),
        submitted_shnarf_last_block_hashes={_END_SHNARF: _END_LAST_BLOCK_HASH},
        approved_vks=set(approved_vks),
    )


def _base_submission(program_vks) -> FinalizationSubmission:
    """
    A finalization submission carrying the single combined `program_vks` list
    nested in the PI (order bound to the proof). Empty `l2_l1_roots` /
    `filtered_addresses` keep the preimage-hash checks trivial (their keccak of
    empty input is the PI hash), and the FTX/rolling-hash boundary values are
    held constant across parent/end so continuity passes without any FTX deadline
    machinery.
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
        parent_processed_ftx_number=U64(7),
        end_ftx_rolling_hash=_FTX_ROLLING_HASH,
        end_processed_ftx_number=U64(7),
        filtered_addresses_hash=hash_address_list([]),
        parent_shnarf=_PARENT_SHNARF,
        end_shnarf=_END_SHNARF,
        program_vks=list(program_vks),
    )
    return FinalizationSubmission(
        public_inputs=pi,
        proof=b"",
        l2_l1_roots=[],
        filtered_addresses=[],
        l2_messaging_blocks_offsets=[],
    )


def test_finalize_rollup_rejects_unapproved_vk() -> None:
    # One committed VK (0xbb, rollup-origin) is NOT on the approved list — the
    # "operator swapped in an unapproved guest" case. L1 does not distinguish
    # exec vs rollup, so the single generic check rejects it.
    state = _base_state(approved_vks={_EXEC_VK_A})
    submission = _base_submission(program_vks=[_EXEC_VK_A, _ROLLUP_VK])
    with pytest.raises(Exception, match="program VK is not approved"):
        finalize_rollup(state, submission)
    # Finalization reverted: state is unchanged.
    assert state.current_finalized_shnarf == _PARENT_SHNARF


def test_finalize_rollup_accepts_two_approved_exec_vks() -> None:
    # Goal-2b / multi-version finalization: a single finalization whose rollup
    # proofs carried TWO different (both approved) exec VKs — two forks
    # aggregated together, aggregation-grained — must succeed. The `program_vks`
    # PI is the canonical sorted-distinct set, so the input is sorted ascending
    # by byte value: 0xA1 (_EXEC_VK_B) < 0xAA (_EXEC_VK_A) < 0xBB (_ROLLUP_VK).
    state = _base_state(approved_vks={_EXEC_VK_A, _EXEC_VK_B, _ROLLUP_VK})
    submission = _base_submission(program_vks=[_EXEC_VK_B, _EXEC_VK_A, _ROLLUP_VK])
    finalize_rollup(state, submission)  # must not raise
    # Finalization applied: shnarf and block number advanced.
    assert state.current_finalized_shnarf == _END_SHNARF
    assert state.current_finalized_last_block_hash == _END_LAST_BLOCK_HASH
    assert int(state.current_l2_block_number) == 1000520


def test_finalize_rollup_succeeds_when_all_vks_approved() -> None:
    # No-regression: the ordinary single-exec-VK + rollup-VK happy path still
    # finalizes when every committed VK is approved.
    state = _base_state(approved_vks={_EXEC_VK_A, _ROLLUP_VK})
    submission = _base_submission(program_vks=[_EXEC_VK_A, _ROLLUP_VK])
    finalize_rollup(state, submission)  # must not raise
    assert state.current_finalized_shnarf == _END_SHNARF
    assert int(state.current_l2_block_number) == 1000520
