import pytest
from ethereum.crypto.hash import Hash32, keccak256
from ethereum_types.numeric import U64

from rollup_spec.l1_rollup import (
    FinalizationSubmission,
    LinethRollupState,
    PlonkVerifier,
    _encode_offset,
    finalize_rollup,
)
from rollup_spec.rollup import RollupPublicInput


_PREV_DATA_HASH = Hash32(bytes([0x11]) * 32)
_PREV_BLOCK_HASH = Hash32(bytes([0x22]) * 32)
_BRIDGE_HASH = Hash32(bytes([0x33]) * 32)
_FTX_HASH = Hash32(bytes([0x44]) * 32)
_CONFIG_HASH = Hash32(bytes([0x55]) * 32)


def _state(prev_offset: int) -> LinethRollupState:
    return LinethRollupState(
        current_finalized_position_commitment=Hash32(
            keccak256(_PREV_DATA_HASH + _encode_offset(prev_offset)),
        ),
        current_finalized_last_block_hash=_PREV_BLOCK_HASH,
        current_l2_block_number=U64(1),
        current_l2_block_timestamp=U64(1),
        current_finalized_l1_l2_bridge_rolling_hash=_BRIDGE_HASH,
        current_finalized_l1_l2_bridge_rolling_hash_message_number=U64(0),
        current_finalized_ftx_rolling_hash=_FTX_HASH,
        current_finalized_processed_ftx_number=U64(0),
        verifier=PlonkVerifier(_CONFIG_HASH),
    )


def _submission(start_offset: int) -> FinalizationSubmission:
    empty_hash = Hash32(keccak256(b""))
    public_inputs = RollupPublicInput(
        end_block_number=U64(2),
        end_block_timestamp=U64(2),
        l2_l1_bridge_transaction_tree=empty_hash,
        parent_l1_l2_bridge_rolling_hash=_BRIDGE_HASH,
        parent_l1_l2_bridge_rolling_hash_message_number=U64(0),
        end_l1_l2_bridge_rolling_hash=_BRIDGE_HASH,
        end_l1_l2_bridge_rolling_hash_message_number=U64(0),
        dynamic_chain_config_hash=_CONFIG_HASH,
        parent_ftx_rolling_hash=_FTX_HASH,
        parent_ftx_number=U64(0),
        end_ftx_rolling_hash=_FTX_HASH,
        end_processed_ftx_number=U64(0),
        filtered_addresses_hash=empty_hash,
        parent_data_rolling_hash=_PREV_DATA_HASH,
        end_data_rolling_hash=Hash32(bytes([0x66]) * 32),
        parent_block_hash=_PREV_BLOCK_HASH,
        end_block_hash=Hash32(bytes([0x77]) * 32),
        start_offset=start_offset,
        end_offset=0,
        program_vks=[],
    )
    return FinalizationSubmission(public_inputs, b"", [], [])


def test_finalization_rejects_zero_start_offset_when_previous_position_is_inside_blob() -> None:
    prev_offset = 9

    with pytest.raises(Exception, match="startOffset does not match the finalized position"):
        finalize_rollup(_state(prev_offset), _submission(start_offset=0), _PREV_DATA_HASH, prev_offset)
