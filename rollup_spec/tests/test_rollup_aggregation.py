"""
Unit tests for `assert_rollup_proof_continuity` (rollup_aggregation.py).

Each test isolates one continuity check: `left`/`right` are built seam-
consistent by default (`_left_pi()`/`_right_pi()` share matching parent/end
values at every checked field), then a single test overrides exactly the
field under test.

Run from the rollup_spec/ directory:  python -m pytest
"""

from dataclasses import replace

import pytest

from ethereum.crypto.hash import Hash32
from ethereum_types.numeric import U64

from rollup_spec.rollup import BLOB_BYTES_LENGTH, RollupProof, RollupPublicInput
from rollup_spec.rollup_aggregation import assert_rollup_proof_continuity


def _base_public_input(**overrides) -> RollupPublicInput:
    base = RollupPublicInput(
        end_block_number=U64(1000510),
        end_block_timestamp=U64(1763000200),
        l2_l1_bridge_transaction_tree=Hash32(bytes([0x11]) * 32),
        parent_l1_l2_bridge_rolling_hash=Hash32(bytes([0x22]) * 32),
        parent_l1_l2_bridge_rolling_hash_message_number=U64(0),
        end_l1_l2_bridge_rolling_hash=Hash32(bytes([0x33]) * 32),
        end_l1_l2_bridge_rolling_hash_message_number=U64(4),
        dynamic_chain_config_hash=Hash32(bytes([0xC0]) * 32),
        parent_ftx_rolling_hash=Hash32(bytes([0x44]) * 32),
        parent_processed_ftx_number=U64(10),
        end_ftx_rolling_hash=Hash32(bytes([0x55]) * 32),
        end_processed_ftx_number=U64(12),
        filtered_addresses_hash=Hash32(bytes([0x66]) * 32),
        parent_drh=Hash32(bytes([0x47]) * 32),
        end_drh=Hash32(bytes([0x8D]) * 32),
        parent_block_hash=Hash32(bytes([0x0A]) * 32),
        end_block_hash=Hash32(bytes([0x0B]) * 32),
        start_offset=0,
        end_offset=131072,
        program_vks=[],
    )
    return replace(base, **overrides)


def _left_pi(**overrides) -> RollupPublicInput:
    # This proof's end-of-range values, which `_right_pi()` mirrors at its
    # start-of-range fields by default.
    defaults = dict(
        end_drh=Hash32(bytes([0x8D]) * 32),
        end_offset=500,
        end_block_hash=Hash32(bytes([0x0B]) * 32),
        end_l1_l2_bridge_rolling_hash=Hash32(bytes([0x33]) * 32),
        end_l1_l2_bridge_rolling_hash_message_number=U64(4),
        end_ftx_rolling_hash=Hash32(bytes([0x55]) * 32),
        end_processed_ftx_number=U64(12),
    )
    defaults.update(overrides)
    return _base_public_input(**defaults)


def _right_pi(**overrides) -> RollupPublicInput:
    defaults = dict(
        parent_drh=Hash32(bytes([0x8D]) * 32),
        start_offset=500,
        parent_block_hash=Hash32(bytes([0x0B]) * 32),
        parent_l1_l2_bridge_rolling_hash=Hash32(bytes([0x33]) * 32),
        parent_l1_l2_bridge_rolling_hash_message_number=U64(4),
        parent_ftx_rolling_hash=Hash32(bytes([0x55]) * 32),
        parent_processed_ftx_number=U64(12),
    )
    defaults.update(overrides)
    return _base_public_input(**defaults)


def _proof(public_inputs: RollupPublicInput) -> RollupProof:
    return RollupProof(public_inputs=public_inputs, start_block_number=U64(1000501))


def test_fully_continuous_proofs_pass() -> None:
    assert_rollup_proof_continuity(_proof(_left_pi()), _proof(_right_pi()))


def test_drh_mismatch_is_rejected() -> None:
    right = _proof(_right_pi(parent_drh=Hash32(bytes([0x99]) * 32)))
    with pytest.raises(Exception, match="DRH continuity"):
        assert_rollup_proof_continuity(_proof(_left_pi()), right)


def test_offset_mismatch_is_rejected() -> None:
    right = _proof(_right_pi(start_offset=501))
    with pytest.raises(Exception, match="offset continuity"):
        assert_rollup_proof_continuity(_proof(_left_pi()), right)


def test_chunk_boundary_offset_handoff_is_accepted() -> None:
    # A chunk filled exactly to BLOB_BYTES_LENGTH handing off to a proof that
    # naturally starts fresh at offset 0 is the same stream position, even
    # though the two integers differ.
    left = _proof(_left_pi(end_offset=BLOB_BYTES_LENGTH))
    right = _proof(_right_pi(start_offset=0))
    assert_rollup_proof_continuity(left, right)


def test_chunk_boundary_offset_handoff_requires_start_offset_zero() -> None:
    # end_offset == BLOB_BYTES_LENGTH does not excuse an arbitrary mismatched
    # start_offset — only the (BLOB_BYTES_LENGTH, 0) pair is the special case.
    left = _proof(_left_pi(end_offset=BLOB_BYTES_LENGTH))
    right = _proof(_right_pi(start_offset=5))
    with pytest.raises(Exception, match="offset continuity"):
        assert_rollup_proof_continuity(left, right)


def test_block_hash_mismatch_is_rejected() -> None:
    right = _proof(_right_pi(parent_block_hash=Hash32(bytes([0x99]) * 32)))
    with pytest.raises(Exception, match="block-hash continuity"):
        assert_rollup_proof_continuity(_proof(_left_pi()), right)


def test_l1_l2_rolling_hash_mismatch_is_rejected() -> None:
    right = _proof(_right_pi(parent_l1_l2_bridge_rolling_hash=Hash32(bytes([0x99]) * 32)))
    with pytest.raises(Exception, match="L1-to-L2 rolling-hash continuity"):
        assert_rollup_proof_continuity(_proof(_left_pi()), right)


def test_l1_l2_rolling_hash_message_number_mismatch_is_rejected() -> None:
    right = _proof(_right_pi(parent_l1_l2_bridge_rolling_hash_message_number=U64(999)))
    with pytest.raises(Exception, match="rolling-hash-number continuity"):
        assert_rollup_proof_continuity(_proof(_left_pi()), right)


def test_dynamic_chain_config_mismatch_is_rejected() -> None:
    right = _proof(_right_pi(dynamic_chain_config_hash=Hash32(bytes([0x99]) * 32)))
    with pytest.raises(Exception, match="dynamic chain configuration continuity"):
        assert_rollup_proof_continuity(_proof(_left_pi()), right)


def test_ftx_rolling_hash_mismatch_is_rejected() -> None:
    right = _proof(_right_pi(parent_ftx_rolling_hash=Hash32(bytes([0x99]) * 32)))
    with pytest.raises(Exception, match="FTX rolling-hash continuity"):
        assert_rollup_proof_continuity(_proof(_left_pi()), right)


def test_ftx_processed_number_mismatch_is_rejected() -> None:
    right = _proof(_right_pi(parent_processed_ftx_number=U64(999)))
    with pytest.raises(Exception, match="processed-FTX-number continuity"):
        assert_rollup_proof_continuity(_proof(_left_pi()), right)
