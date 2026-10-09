"""
Tests for `scan_bridge_logs`, the l2-execution guest's L2MessageService log handling:
L2->L1 message collection with 1-based messaging block offsets, the L1->L2 rolling-hash
pair derived from `RollingHashUpdated` logs, and the "no L2MessageService configured"
(zero-address) path that leaves the parent pair untouched.

Run from the rollup_spec/ directory:  python -m pytest
"""

import pytest
from ethereum.crypto.hash import Hash32
from ethereum.state import Address
from ethereum_types.numeric import U64

from rollup_spec.fork import Log
from rollup_spec.l2_execution import (
    BRIDGE_L1L2_ROLLING_HASH_UPDATED_TOPIC_0,
    BRIDGE_L2L1_MESSAGE_SENT_TOPIC_0,
    ZERO_ADDRESS,
    ZERO_HASH,
    BridgeLogScan,
    scan_bridge_logs,
)

_MESSAGE_SERVICE = Address(bytes([0x11]) * 20)
_PARENT_ROLLING_HASH = Hash32(bytes([0x0B]) * 32)
_PARENT_MESSAGE_NUMBER = U64(7)


def _message_sent(address: Address, message_hash: Hash32) -> Log:
    """An L2MessageService `MessageSent` log carrying `message_hash` as its fourth topic."""
    return Log(address, (BRIDGE_L2L1_MESSAGE_SENT_TOPIC_0, ZERO_HASH, ZERO_HASH, message_hash), b"")


def _rolling_hash_updated(address: Address, message_number: int, rolling_hash: Hash32) -> Log:
    """An L2MessageService `RollingHashUpdated(uint256 indexed, bytes32 indexed)` log."""
    return Log(
        address,
        (
            BRIDGE_L1L2_ROLLING_HASH_UPDATED_TOPIC_0,
            Hash32(message_number.to_bytes(32, "big")),
            rolling_hash,
        ),
        b"",
    )


def _scan(block_logs: list[list[Log]], address: Address = _MESSAGE_SERVICE) -> BridgeLogScan:
    """Scan `block_logs` against the shared parent bridge pair."""
    return scan_bridge_logs(address, _PARENT_ROLLING_HASH, _PARENT_MESSAGE_NUMBER, block_logs)


# ── zero L2MessageService address ───────────────────────────────────────────────


def test_zero_address_ignores_bridge_logs_and_passes_the_parent_pair_through() -> None:
    logs = [
        _message_sent(ZERO_ADDRESS, Hash32(bytes([0xAB]) * 32)),
        _rolling_hash_updated(ZERO_ADDRESS, 9, Hash32(bytes([0xE1]) * 32)),
    ]

    scan = _scan([logs], address=ZERO_ADDRESS)

    assert scan == BridgeLogScan([], [], _PARENT_ROLLING_HASH, _PARENT_MESSAGE_NUMBER)


# ── L2->L1 messages from MessageSent logs ───────────────────────────────────────


def test_messages_are_collected_in_order_with_the_one_based_offsets_of_the_blocks_that_emitted_them() -> None:
    messages = [Hash32(bytes([value]) * 32) for value in (0xAB, 0xAC, 0xCD)]

    scan = _scan([
        [_message_sent(_MESSAGE_SERVICE, messages[0]), _message_sent(_MESSAGE_SERVICE, messages[1])],
        [],
        [_message_sent(_MESSAGE_SERVICE, messages[2])],
    ])

    assert scan.l2_l1_messages == messages
    assert scan.l2_messaging_blocks_offsets == [1, 3]


# ── L1->L2 rolling-hash pair from RollingHashUpdated logs ───────────────────────


def test_bridge_pair_ends_at_the_parent_pair_when_the_range_emits_no_rolling_hash_update() -> None:
    scan = _scan([[]])

    assert scan.end_l1_l2_bridge_rolling_hash == _PARENT_ROLLING_HASH
    assert scan.end_l1_l2_bridge_rolling_hash_message_number == _PARENT_MESSAGE_NUMBER


def test_bridge_pair_ends_at_the_last_rolling_hash_update_of_a_multi_block_range() -> None:
    last_rolling_hash = Hash32(bytes([0xE2]) * 32)

    scan = _scan([
        [_rolling_hash_updated(_MESSAGE_SERVICE, 12, Hash32(bytes([0xE1]) * 32))],
        [_rolling_hash_updated(_MESSAGE_SERVICE, 15, last_rolling_hash)],
    ])

    assert scan.end_l1_l2_bridge_rolling_hash == last_rolling_hash
    assert scan.end_l1_l2_bridge_rolling_hash_message_number == U64(15)


def test_rolling_hash_update_must_advance_past_the_current_message_number() -> None:
    logs = [_rolling_hash_updated(_MESSAGE_SERVICE, int(_PARENT_MESSAGE_NUMBER), Hash32(bytes([0xE1]) * 32))]

    with pytest.raises(Exception, match="message number must increase"):
        _scan([logs])
