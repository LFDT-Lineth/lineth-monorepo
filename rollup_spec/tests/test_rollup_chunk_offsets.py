"""Guest-level DA ownership and chunk binding scenarios."""

from dataclasses import replace

import pytest
import zstandard as zstd
from ethereum.crypto.hash import Hash32, keccak256
from ethereum.state import Address
from ethereum_types.numeric import U64

from rollup_spec import rollup
from rollup_spec.l2_execution import (
    L2ExecutionProof,
    L2ExecutionProofPublicInput,
    VerifiableL2ExecutionProof,
    hash_address_list,
)
from rollup_spec.rollup import (
    BLOB_PAYLOAD_CAPACITY,
    ChunkWitness,
    ConflationWitness,
    RollupProofPrivateInput,
    collect_l2_l1_messages,
    pack_blob_payload,
    run_rollup_guest,
)

ZERO = Hash32(bytes(32))
PAYLOAD = b"canonical truncated-block RLP"


def _segment(payload=PAYLOAD):
    frame = zstd.ZstdCompressor().compress(payload)
    return len(frame).to_bytes(4, "big") + frame


def _blob(monkeypatch, payload):
    physical = pack_blob_payload(payload)
    # Mock only the external KZG primitive; retain physical packing, parsing,
    # hashing, ownership and execution-continuity checks in the guest.
    monkeypatch.setattr(rollup, "_trusted_setup", lambda: object())
    monkeypatch.setattr(rollup.ckzg, "blob_to_kzg_commitment", lambda blob, setup: keccak256(blob) + bytes(16))
    monkeypatch.setattr(rollup, "kzg_commitment_to_versioned_hash", lambda commitment: bytes(commitment)[:32])
    return ChunkWitness(Hash32(keccak256(physical)), False, blob_bytes=physical)


def _calldata(data):
    return ChunkWitness(Hash32(keccak256(data)), True, calldata_bytes=data)


def _input(monkeypatch, chunks, count=1, start_offset=0):
    empty_addresses = hash_address_list([])
    empty_messages = []
    pi = L2ExecutionProofPublicInput(
        parent_block_hash=ZERO, end_block_hash=ZERO, end_block_number=U64(1),
        end_block_timestamp=U64(1), l2_l1_messages=empty_messages,
        parent_l1_l2_bridge_rolling_hash=ZERO, parent_l1_l2_bridge_rolling_hash_message_number=U64(0),
        end_l1_l2_bridge_rolling_hash=ZERO, end_l1_l2_bridge_rolling_hash_message_number=U64(0),
        dynamic_chain_config_hash=ZERO, parent_ftx_rolling_hash=ZERO, parent_ftx_number=U64(0),
        end_ftx_rolling_hash=ZERO, end_processed_ftx_number=U64(0),
        filtered_addresses_hash=empty_addresses, tx_froms_hash=empty_addresses, block_count=1,
    )
    proofs = [VerifiableL2ExecutionProof(
        L2ExecutionProof(replace(pi, end_block_number=U64(i)), U64(i)), ZERO,
    ) for i in range(1, count + 1)]
    monkeypatch.setattr(rollup, "_truncate_conflation", lambda blocks, chain_id: ([replace_dummy], [ZERO]))
    monkeypatch.setattr(rollup, "rlp_encode_truncated_blocks", lambda blocks: PAYLOAD)
    previous = ZERO
    if start_offset:
        previous = Hash32(keccak256(ZERO + chunks[0].chunk_hash))
    return RollupProofPrivateInput(
        previous, start_offset, U64(1), [ConflationWitness([b"block"]) for _ in range(count)],
        chunks, proofs,
        boundary_prev_data_rolling_hash=ZERO if start_offset else None,
    )


replace_dummy = type("Block", (), {"block_hash": ZERO, "froms": []})()


def test_shared_blob_prefix_and_suffix_are_read_from_same_physical_bytes(monkeypatch):
    segment = _segment()
    prefix, suffix = b"prior proof", b"next proof"
    chunk = _blob(monkeypatch, prefix + segment + suffix)
    proof = run_rollup_guest(_input(monkeypatch, [chunk], start_offset=len(prefix)))
    assert proof.public_inputs.start_offset == len(prefix)
    assert proof.public_inputs.end_offset == len(prefix) + len(segment)
    assert proof.public_inputs.end_data_rolling_hash == proof.public_inputs.parent_data_rolling_hash


@pytest.mark.parametrize("split", [2, 7])
def test_frame_prefix_or_body_crosses_full_blob_boundary(monkeypatch, split):
    segment = _segment()
    prefix = bytes(BLOB_PAYLOAD_CAPACITY - split)
    first = _blob(monkeypatch, prefix + segment[:split])
    second = _blob(monkeypatch, segment[split:])
    proof = run_rollup_guest(_input(monkeypatch, [first, second], start_offset=len(prefix)))
    assert proof.public_inputs.end_offset == 0  # last short blob completely consumed
    assert proof.public_inputs.end_data_rolling_hash == Hash32(keccak256(proof.public_inputs.parent_data_rolling_hash + second.chunk_hash))


def test_calldata_mixed_with_blob_parses_complete_segments(monkeypatch):
    segment = _segment()
    first = _calldata(segment)
    second = _blob(monkeypatch, segment)
    proof = run_rollup_guest(_input(monkeypatch, [first, second], count=2))
    assert proof.public_inputs.end_offset == 0


def test_collect_l2_l1_messages_preserves_execution_order():
    first = [Hash32(i.to_bytes(32, "big")) for i in range(1, 21)]
    second = [Hash32(i.to_bytes(32, "big")) for i in range(21, 41)]
    assert collect_l2_l1_messages([first, [], second]) == first + second
    assert collect_l2_l1_messages([[], []]) == []


def test_rollup_binds_sender_hash_to_each_conflation(monkeypatch):
    sender = Address(bytes([0x23]) * 20)
    rollup_input = _input(monkeypatch, [_calldata(_segment()), _calldata(_segment())], count=2)
    calls = iter([0, 1])

    def truncated(blocks, chain_id):
        froms = [sender] if next(calls) == 1 else []
        return [type("Block", (), {"block_hash": ZERO, "froms": froms})()], [ZERO]

    monkeypatch.setattr(rollup, "_truncate_conflation", truncated)
    rollup_input.l2_execution_proofs[1].proof.public_inputs.tx_froms_hash = hash_address_list([sender])
    run_rollup_guest(rollup_input)


def test_rollup_rejects_sender_hash_mismatch_for_conflation(monkeypatch):
    sender = Address(bytes([0x23]) * 20)
    rollup_input = _input(monkeypatch, [_calldata(_segment()), _calldata(_segment())], count=2)
    calls = iter([0, 1])

    def truncated(blocks, chain_id):
        froms = [sender] if next(calls) == 1 else []
        return [type("Block", (), {"block_hash": ZERO, "froms": froms})()], [ZERO]

    monkeypatch.setattr(rollup, "_truncate_conflation", truncated)
    with pytest.raises(Exception, match="txFromsHash does not match DA block senders"):
        run_rollup_guest(rollup_input)


def test_calldata_requires_frame_alignment_and_no_trailing_bytes(monkeypatch):
    segment = _segment()
    with pytest.raises(Exception, match="segment boundaries"):
        run_rollup_guest(_input(monkeypatch, [_calldata(segment[:5]), _calldata(segment[5:])]))
    with pytest.raises(Exception, match="trailing bytes"):
        run_rollup_guest(_input(monkeypatch, [_calldata(segment + b"extra")]))


def test_chunk_rejects_invalid_physical_blob_and_wrong_binding_hash(monkeypatch):
    valid = _blob(monkeypatch, _segment())
    with pytest.raises(Exception, match="invalid physical blob"):
        run_rollup_guest(_input(monkeypatch, [replace(valid, blob_bytes=valid.blob_bytes[:-1])]))
    with pytest.raises(Exception, match="chunkHash"):
        run_rollup_guest(_input(monkeypatch, [replace(valid, chunk_hash=ZERO)]))
    with pytest.raises(Exception, match="chunkHash"):
        run_rollup_guest(_input(monkeypatch, [replace(_calldata(_segment()), chunk_hash=ZERO)]))


def test_nonterminal_blob_must_be_full_and_every_chunk_owned(monkeypatch):
    segment = _segment()
    with pytest.raises(Exception, match="full payload"):
        run_rollup_guest(_input(monkeypatch, [_blob(monkeypatch, segment), _calldata(segment)], count=2))
    prefix = bytes(BLOB_PAYLOAD_CAPACITY - len(segment))
    with pytest.raises(Exception, match="must contain owned bytes"):
        run_rollup_guest(_input(monkeypatch, [_blob(monkeypatch, segment + prefix), _blob(monkeypatch, b"")]))
