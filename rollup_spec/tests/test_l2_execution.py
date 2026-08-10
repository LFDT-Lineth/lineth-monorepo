"""
Tests for the l2-execution guest's "no L2MessageService configured" (zero-address)
bridge-suppression path.

The zero-address path is what lets a vanilla stateless input (no L2MessageService
account, witness covering only what execution touched) run through the extended
guest unchanged — it is the reference-side counterpart of the guest's
`bridge_suppressed` branch.

Run from the rollup_spec/ directory:  python -m pytest
"""

from pathlib import Path

from ethereum.crypto.hash import Hash32, keccak256
from ethereum.state import Address
from ethereum_types.numeric import U64

from rollup_spec import l2_execution
from rollup_spec.block import ChainConfig, LinethPayloadInput
from rollup_spec.fork import Log
from rollup_spec.l2_execution import (
    BRIDGE_L2L1_MESSAGE_SENT_TOPIC_0,
    ZERO_ADDRESS,
    ZERO_HASH,
    L2ExecutionProofPrivateInput,
    read_l1l2_bridge_state,
    run_l2_execution_guest,
)
from rollup_spec.proof_io_v1 import decode_request_json
from rollup_spec.state_transition import (
    EMPTY_TRIE_ROOT_HASH,
    L2State,
    StatelessExecutionResult,
)
from rollup_spec.stateless_input import decode_stateless_input_ssz

_TESTDATA_DIR = Path(l2_execution.__file__).resolve().parent / "prover_io" / "testdata"


def _golden_vanilla_stateless_input_ssz() -> bytes:
    """A real, valid vanilla stateless-input SSZ slice, from the golden JSON request."""
    request = (_TESTDATA_DIR / "getZkL2ExecutionProofV1.request.json").read_text()
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
        ),
    )


# ── read_l1l2_bridge_state: zero-address short-circuit ──────────────────────────


def test_read_l1l2_bridge_state_zero_address_returns_zeros_without_state_access() -> None:
    # A state whose .storage would raise if touched: proves the zero-address guard
    # short-circuits before any MPT read.
    class _ExplodingState:
        def storage(self, *_args, **_kwargs):  # noqa: ANN002, ANN003
            raise AssertionError("storage() must not be called for the zero address")

    rolling_hash, number = read_l1l2_bridge_state(_ExplodingState(), ZERO_ADDRESS)
    assert rolling_hash == ZERO_HASH
    assert int(number) == 0


def test_read_l1l2_bridge_state_nonzero_address_still_reads_state() -> None:
    # Contrast: a non-zero address is NOT suppressed, so the real MPT read runs
    # (here against an empty-trie state, which proves absence -> zero).
    state = L2State(state_root=EMPTY_TRIE_ROOT_HASH, witnesses=[])
    rolling_hash, number = read_l1l2_bridge_state(state, Address(bytes([0x11]) * 20))
    assert rolling_hash == ZERO_HASH  # empty trie => proof of absence => zero
    assert int(number) == 0


# ── run_l2_execution_guest: full zero-address suppression ───────────────────────


def test_run_l2_execution_guest_zero_address_suppresses_bridge_and_messages(monkeypatch) -> None:
    vanilla = _golden_vanilla_stateless_input_ssz()
    ext = _zero_bridge_input(vanilla)

    # A block log that WOULD be collected as an L2->L1 message if the scan ran:
    # its address equals the (zero) configured L2MessageService and topic0 is the
    # bridge signature. Suppression must skip it entirely.
    matching_log = Log(
        address=ZERO_ADDRESS,
        topics=(
            BRIDGE_L2L1_MESSAGE_SENT_TOPIC_0,
            Hash32(b"\x00" * 32),
            Hash32(b"\x00" * 32),
            Hash32(bytes([0xAB]) * 32),
        ),
        data=b"",
    )

    def _fake_execute(stateless_input):  # noqa: ANN001, ANN202
        return StatelessExecutionResult(
            pre_state_root=Hash32(bytes([0x11]) * 32),
            post_state_root=Hash32(bytes([0x22]) * 32),
            block_logs=[matching_log],
        )

    # Boundary + crypto stubs: mock the delegated engine and skip payload-tx sender
    # recovery (empty tx list) so the test needs no real EVM or secp256k1.
    monkeypatch.setattr(l2_execution, "execute_stateless_input", _fake_execute)
    monkeypatch.setattr(l2_execution, "parse_payload_transaction_rlps", lambda payload: [])

    proof = run_l2_execution_guest(ext)
    pi = proof.public_inputs

    # All four bridge PI fields pinned to zero.
    assert pi.parent_l1_l2_bridge_rolling_hash == ZERO_HASH
    assert int(pi.parent_l1_l2_bridge_rolling_hash_message_number) == 0
    assert pi.end_l1_l2_bridge_rolling_hash == ZERO_HASH
    assert int(pi.end_l1_l2_bridge_rolling_hash_message_number) == 0

    # L2->L1 message scan skipped despite the matching log present.
    assert proof.l2_l1_messages == []
    assert pi.l2_l1_messages_hash == Hash32(keccak256(b""))
