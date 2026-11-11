"""
Tests for the l2-execution guest's L2MessageService log scan and its "no
L2MessageService configured" (zero-address) bridge-suppression path.

The zero-address path is what lets a vanilla stateless input (no L2MessageService
account, witness covering only what execution touched) run through the extended
guest unchanged — it is the reference-side counterpart of the guest's
`bridge_suppressed` branch.

Run from the rollup_spec/ directory:  python -m pytest
"""

from pathlib import Path

import pytest
from ethereum.crypto.hash import Hash32, keccak256
from ethereum.state import Address
from ethereum_types.numeric import U64

from rollup_spec import l2_execution
from rollup_spec.block import (
    MAINNET_BEACON_ROOTS_ADDRESS,
    MAINNET_HISTORY_STORAGE_ADDRESS,
    ChainConfig,
    LinethPayloadInput,
    SystemContracts,
)
from rollup_spec.fork import Log
from rollup_spec.l2_execution import (
    BRIDGE_L2L1_MESSAGE_SENT_TOPIC_0,
    BRIDGE_ROLLING_HASH_UPDATED_TOPIC_0,
    ZERO_ADDRESS,
    ZERO_HASH,
    L2ExecutionProofPrivateInput,
    last_non_zero_pair,
    run_l2_execution_guest,
    scan_l2_message_service_logs,
)
from rollup_spec.proof_io_v1 import decode_request_json
from rollup_spec.state_transition import StatelessExecutionResult
from rollup_spec.stateless_input import decode_stateless_input_ssz

_TESTDATA_DIR = Path(l2_execution.__file__).resolve().parent / "prover_io" / "testdata"

_MESSAGE_SERVICE = Address(bytes([0x11]) * 20)
_OTHER_CONTRACT = Address(bytes([0x22]) * 20)

# Distinct per-field addresses so a field swap anywhere in the pipeline is caught. Beacon
# roots and history storage must be the mainnet addresses for the guest to accept them.
_SYSTEM_CONTRACTS = SystemContracts(
    beacon_roots=MAINNET_BEACON_ROOTS_ADDRESS,
    history_storage=MAINNET_HISTORY_STORAGE_ADDRESS,
    deposit=Address(bytes([0xA3]) * 20),
    withdrawal_request=Address(bytes([0xA4]) * 20),
    consolidation_request=Address(bytes([0xA5]) * 20),
    builder_deposit_request=Address(bytes([0xA6]) * 20),
    builder_exit_request=Address(bytes([0xA7]) * 20),
)


def _fixture(name: str) -> Path:
    """Resolve `<name>.json`, allowing an optional `<startBlock>-<endBlock>-` prefix."""
    matches = sorted(_TESTDATA_DIR.glob(f"*{name}"))
    assert matches, f"no fixture matching *{name} in {_TESTDATA_DIR}"
    assert len(matches) == 1, f"multiple fixtures matching *{name}: {matches}"
    return matches[0]


def _golden_vanilla_stateless_input_ssz() -> bytes:
    """A real, valid vanilla stateless-input SSZ slice, from the golden JSON request."""
    request = _fixture("getZkL2ExecutionProofV1.request.json").read_text()
    return decode_request_json(request).payloads[0].stateless_input_ssz


def _zero_bridge_input(vanilla: bytes) -> L2ExecutionProofPrivateInput:
    """
    Test-local setup: a single-payload extended input around a vanilla slice with a
    zero `l2_message_service_address`, so the guest's bridge-suppression branch runs.
    `chain_id`/`coinbase` are read off the vanilla input so the conflation invariants
    (chain-id match, `feeRecipient == coinbase`) hold.
    """
    si = decode_stateless_input_ssz(vanilla)
    return L2ExecutionProofPrivateInput(
        parent_ftx_rolling_hash=ZERO_HASH,
        parent_last_processed_ftx_number=U64(0),
        payloads=[LinethPayloadInput(stateless_input_ssz=vanilla)],
        chain_config=ChainConfig(
            l2_message_service_address=ZERO_ADDRESS,
            coinbase=si.new_payload_request.execution_payload.fee_recipient,
            chain_id=si.chain_config.chain_id,
            system_contracts=_SYSTEM_CONTRACTS,
        ),
    )


def _message_sent(address: Address, message_hash: Hash32) -> Log:
    return Log(address, (BRIDGE_L2L1_MESSAGE_SENT_TOPIC_0, ZERO_HASH, ZERO_HASH, message_hash), b"")


def _rolling_hash_updated(address: Address, message_number: int, rolling_hash: Hash32) -> Log:
    topics = (BRIDGE_ROLLING_HASH_UPDATED_TOPIC_0, Hash32(message_number.to_bytes(32, "big")), rolling_hash)
    return Log(address, topics, b"")


def _hash(value: int) -> Hash32:
    return Hash32(bytes([value]) * 32)


# ── event signatures ────────────────────────────────────────────────────────────


def test_rolling_hash_updated_topic_is_the_event_signature_hash() -> None:
    assert BRIDGE_ROLLING_HASH_UPDATED_TOPIC_0 == bytes.fromhex(
        "99b65a4301b38c09fb6a5f27052d73e8372bbe8f6779d678bfe8a41b66cce7ac"
    )


# ── scan_l2_message_service_logs ────────────────────────────────────────────────


def test_scan_of_a_block_without_logs_yields_no_messages_and_the_zero_pair() -> None:
    messages, pair = scan_l2_message_service_logs([], _MESSAGE_SERVICE)
    assert messages == []
    assert pair == (ZERO_HASH, U64(0))


def test_scan_yields_the_pair_of_a_single_rolling_hash_updated_log() -> None:
    logs = [_rolling_hash_updated(_MESSAGE_SERVICE, 7, _hash(0xA1))]
    messages, pair = scan_l2_message_service_logs(logs, _MESSAGE_SERVICE)
    assert messages == []
    assert pair == (_hash(0xA1), U64(7))


def test_scan_yields_the_pair_of_the_last_rolling_hash_updated_log() -> None:
    logs = [
        _rolling_hash_updated(_MESSAGE_SERVICE, 7, _hash(0xA1)),
        _rolling_hash_updated(_MESSAGE_SERVICE, 12, _hash(0xA2)),
        _rolling_hash_updated(_MESSAGE_SERVICE, 15, _hash(0xA3)),
    ]
    _, pair = scan_l2_message_service_logs(logs, _MESSAGE_SERVICE)
    assert pair == (_hash(0xA3), U64(15))


def test_scan_ignores_logs_emitted_by_other_addresses() -> None:
    logs = [
        _rolling_hash_updated(_MESSAGE_SERVICE, 7, _hash(0xA1)),
        _rolling_hash_updated(_OTHER_CONTRACT, 9, _hash(0xA2)),
        _message_sent(_OTHER_CONTRACT, _hash(0xB1)),
    ]
    messages, pair = scan_l2_message_service_logs(logs, _MESSAGE_SERVICE)
    assert messages == []
    assert pair == (_hash(0xA1), U64(7))


def test_scan_ignores_logs_without_a_matching_event_signature() -> None:
    logs = [
        Log(_MESSAGE_SERVICE, (), b""),
        Log(_MESSAGE_SERVICE, (_hash(0xEE), _hash(0x01), _hash(0x02), _hash(0x03)), b""),
    ]
    messages, pair = scan_l2_message_service_logs(logs, _MESSAGE_SERVICE)
    assert messages == []
    assert pair == (ZERO_HASH, U64(0))


@pytest.mark.parametrize("topic_count", [1, 2])
def test_scan_rejects_a_rolling_hash_updated_log_with_fewer_than_three_topics(topic_count: int) -> None:
    topics = (BRIDGE_ROLLING_HASH_UPDATED_TOPIC_0, Hash32((7).to_bytes(32, "big")))[:topic_count]
    with pytest.raises(Exception, match="RollingHashUpdated log is missing"):
        scan_l2_message_service_logs([Log(_MESSAGE_SERVICE, topics, b"")], _MESSAGE_SERVICE)


def test_scan_accepts_the_largest_uint64_message_number() -> None:
    logs = [_rolling_hash_updated(_MESSAGE_SERVICE, 2**64 - 1, _hash(0xA1))]
    _, pair = scan_l2_message_service_logs(logs, _MESSAGE_SERVICE)
    assert pair == (_hash(0xA1), U64(2**64 - 1))


@pytest.mark.parametrize("message_number", [2**64, 2**255])
def test_scan_rejects_a_message_number_that_does_not_fit_uint64(message_number: int) -> None:
    logs = [_rolling_hash_updated(_MESSAGE_SERVICE, message_number, _hash(0xA1))]
    with pytest.raises(Exception, match="exceeds uint64"):
        scan_l2_message_service_logs(logs, _MESSAGE_SERVICE)


def test_scan_extracts_messages_alongside_rolling_hash_updates() -> None:
    logs = [
        _message_sent(_MESSAGE_SERVICE, _hash(0xB1)),
        _rolling_hash_updated(_MESSAGE_SERVICE, 3, _hash(0xA1)),
        _message_sent(_MESSAGE_SERVICE, _hash(0xB2)),
        _rolling_hash_updated(_MESSAGE_SERVICE, 5, _hash(0xA2)),
    ]
    messages, pair = scan_l2_message_service_logs(logs, _MESSAGE_SERVICE)
    assert messages == [_hash(0xB1), _hash(0xB2)]
    assert pair == (_hash(0xA2), U64(5))


def test_scan_rejects_a_message_sent_log_missing_its_hash_topic() -> None:
    log = Log(_MESSAGE_SERVICE, (BRIDGE_L2L1_MESSAGE_SENT_TOPIC_0,), b"")
    with pytest.raises(Exception, match="missing its message hash topic"):
        scan_l2_message_service_logs([log], _MESSAGE_SERVICE)


# ── last_non_zero_pair ──────────────────────────────────────────────────────────


def test_last_non_zero_pair_of_no_pairs_is_zero() -> None:
    assert last_non_zero_pair([]) == (ZERO_HASH, U64(0))


def test_last_non_zero_pair_of_only_zero_pairs_is_zero() -> None:
    assert last_non_zero_pair([(ZERO_HASH, U64(0))] * 3) == (ZERO_HASH, U64(0))


def test_last_non_zero_pair_skips_trailing_zero_pairs() -> None:
    pairs = [(_hash(0xA1), U64(3)), (_hash(0xA2), U64(5)), (ZERO_HASH, U64(0)), (ZERO_HASH, U64(0))]
    assert last_non_zero_pair(pairs) == (_hash(0xA2), U64(5))


def test_last_non_zero_pair_prefers_a_later_non_zero_pair_over_leading_ones() -> None:
    pairs = [(ZERO_HASH, U64(0)), (_hash(0xA1), U64(3)), (ZERO_HASH, U64(0)), (_hash(0xA3), U64(8))]
    assert last_non_zero_pair(pairs) == (_hash(0xA3), U64(8))


# ── run_l2_execution_guest: full zero-address suppression ───────────────────────


def test_run_l2_execution_guest_zero_address_suppresses_bridge_and_messages(monkeypatch) -> None:
    vanilla = _golden_vanilla_stateless_input_ssz()
    ext = _zero_bridge_input(vanilla)

    # Block logs that WOULD be collected if the scan ran: their address equals the
    # (zero) configured L2MessageService. Suppression must skip them entirely.
    matching_logs = [
        _message_sent(ZERO_ADDRESS, _hash(0xAB)),
        _rolling_hash_updated(ZERO_ADDRESS, 3, _hash(0xA1)),
    ]

    def _fake_execute(stateless_input, system_contracts):  # noqa: ANN001, ANN202
        assert system_contracts == _SYSTEM_CONTRACTS
        return StatelessExecutionResult(
            pre_state_root=Hash32(bytes([0x11]) * 32),
            post_state_root=Hash32(bytes([0x22]) * 32),
            block_logs=matching_logs,
        )

    # Boundary + crypto stubs: mock the delegated engine and skip payload-tx sender
    # recovery (empty tx list) so the test needs no real EVM or secp256k1.
    monkeypatch.setattr(l2_execution, "execute_stateless_input", _fake_execute)
    monkeypatch.setattr(l2_execution, "parse_payload_transaction_rlps", lambda payload: [])

    proof = run_l2_execution_guest(ext)
    pi = proof.public_inputs

    assert pi.end_l1_l2_bridge_rolling_hash == ZERO_HASH
    assert int(pi.end_l1_l2_bridge_rolling_hash_message_number) == 0

    # L2->L1 message scan skipped despite the matching log present.
    assert pi.l2_l1_messages == []
    assert pi.block_count == 1
    assert pi.l2_messaging_blocks_offsets == []


def test_execution_emits_messages_offset_and_rolling_hash_pair_of_a_message_service_block(monkeypatch) -> None:
    execution_input = _zero_bridge_input(_golden_vanilla_stateless_input_ssz())
    execution_input.chain_config.l2_message_service_address = _MESSAGE_SERVICE
    logs = [
        _message_sent(_MESSAGE_SERVICE, _hash(0xAB)),
        _rolling_hash_updated(_MESSAGE_SERVICE, 3, _hash(0xA1)),
        _message_sent(_MESSAGE_SERVICE, _hash(0xCD)),
        _rolling_hash_updated(_MESSAGE_SERVICE, 5, _hash(0xA2)),
    ]
    monkeypatch.setattr(l2_execution, "execute_stateless_input", lambda *_: StatelessExecutionResult(
        pre_state_root=Hash32(bytes([0x11]) * 32),
        post_state_root=Hash32(bytes([0x22]) * 32),
        block_logs=logs,
    ))
    monkeypatch.setattr(l2_execution, "parse_payload_transaction_rlps", lambda _: [])

    pi = run_l2_execution_guest(execution_input).public_inputs

    assert pi.l2_l1_messages == [_hash(0xAB), _hash(0xCD)]
    assert pi.block_count == 1
    assert pi.l2_messaging_blocks_offsets == [1]
    assert pi.end_l1_l2_bridge_rolling_hash == _hash(0xA2)
    assert pi.end_l1_l2_bridge_rolling_hash_message_number == U64(5)


# ── run_l2_execution_guest: no blob transactions ────────────────────────────────


class _ReachedExecution(Exception):
    pass


def _input_probing_execution(monkeypatch, versioned_hashes=(), extra_transactions=()) -> L2ExecutionProofPrivateInput:
    """
    A single-payload input around the golden vanilla slice, decoded with `versioned_hashes` and
    `extra_transactions` added, whose delegated execution raises `_ReachedExecution`.
    """
    vanilla = _golden_vanilla_stateless_input_ssz()
    stateless_input = decode_stateless_input_ssz(vanilla)
    stateless_input.new_payload_request.versioned_hashes = list(versioned_hashes)
    stateless_input.new_payload_request.execution_payload.transactions.extend(extra_transactions)

    def _reach_execution(*_):  # noqa: ANN002, ANN202
        raise _ReachedExecution

    monkeypatch.setattr(l2_execution, "decode_stateless_input_ssz", lambda _: stateless_input)
    monkeypatch.setattr(l2_execution, "execute_stateless_input", _reach_execution)
    return _zero_bridge_input(vanilla)


def test_payload_without_blob_transactions_reaches_execution(monkeypatch) -> None:
    with pytest.raises(_ReachedExecution):
        run_l2_execution_guest(_input_probing_execution(monkeypatch))


@pytest.mark.parametrize(
    "versioned_hashes, extra_transactions",
    [
        pytest.param([_hash(0x01)], [], id="blob versioned hash"),
        pytest.param([], [b"\x03" + bytes(31)], id="type-3 transaction"),
    ],
)
def test_payload_with_a_blob_transaction_is_rejected_before_execution(monkeypatch, versioned_hashes, extra_transactions) -> None:
    execution_input = _input_probing_execution(monkeypatch, versioned_hashes, extra_transactions)

    with pytest.raises(Exception, match="blob transactions are not supported"):
        run_l2_execution_guest(execution_input)


# ── dynamicChainConfigHash and the system contracts ───────────────────────────


def _config_with_system_contracts() -> ChainConfig:
    return ChainConfig(
        l2_message_service_address=Address(bytes([0x11]) * 20),
        coinbase=Address(bytes(20)),
        chain_id=U64(59144),
        system_contracts=_SYSTEM_CONTRACTS,
    )


def _word(value: bytes) -> bytes:
    return value.rjust(32, b"\x00")


def test_chain_config_hash_preimage() -> None:
    expected = keccak256(
        _word((59144).to_bytes(8, "big"))
        + _word((1_000_000_000).to_bytes(8, "big"))
        + _word(bytes(20))
        + _word(bytes([0x11]) * 20)
        + b"".join(_word(bytes([byte]) * 20) for byte in (0xA3, 0xA4, 0xA5, 0xA6, 0xA7))
    )
    assert _config_with_system_contracts().hash(1_000_000_000) == expected


def test_chain_config_hash_golden_vector() -> None:
    # Same vector as riscv-guests/l2_execution/test/l2_execution_test.zig, built from the same
    # sample config but with beacon roots/history storage set to 0xA1/0xA2 there (they are not hashed).
    expected = bytes.fromhex("22a4a19c730005a8ec6de4f29e818b679c822d1989b409dbc82bf1526a3eb235")
    assert _config_with_system_contracts().hash(1_000_000_000) == expected


@pytest.mark.parametrize(
    "field",
    ["deposit", "withdrawal_request", "consolidation_request", "builder_deposit_request", "builder_exit_request"],
)
def test_chain_config_hash_commits_to_configurable_system_contracts(field: str) -> None:
    from dataclasses import replace

    config = _config_with_system_contracts()
    tampered = replace(config, system_contracts=replace(_SYSTEM_CONTRACTS, **{field: Address(bytes(20))}))
    assert tampered.hash(7) != config.hash(7)


@pytest.mark.parametrize("field", ["beacon_roots", "history_storage"])
def test_chain_config_hash_excludes_fixed_system_contracts(field: str) -> None:
    from dataclasses import replace

    config = _config_with_system_contracts()
    tampered = replace(config, system_contracts=replace(_SYSTEM_CONTRACTS, **{field: Address(bytes(20))}))
    assert tampered.hash(7) == config.hash(7)


def test_mainnet_fixed_system_contracts_match_the_golden_request() -> None:
    sc = decode_request_json(_fixture("getZkL2ExecutionProofV1.request.json").read_text()).chain_config.system_contracts
    assert sc.beacon_roots == MAINNET_BEACON_ROOTS_ADDRESS
    assert sc.history_storage == MAINNET_HISTORY_STORAGE_ADDRESS


@pytest.mark.parametrize(
    ("field", "message"),
    [
        ("beacon_roots", "beacon roots contract must be the mainnet address"),
        ("history_storage", "history storage contract must be the mainnet address"),
    ],
)
def test_run_l2_execution_guest_rejects_non_mainnet_fixed_system_contracts(field: str, message: str) -> None:
    from dataclasses import replace

    ext = _zero_bridge_input(_golden_vanilla_stateless_input_ssz())
    system_contracts = replace(ext.chain_config.system_contracts, **{field: Address(bytes([0xEE]) * 20)})
    ext = replace(ext, chain_config=replace(ext.chain_config, system_contracts=system_contracts))
    with pytest.raises(Exception, match=message):
        run_l2_execution_guest(ext)
