"""Generate SSZ golden vectors using reference field converters.

Container layouts mirror the guest decoder; remerkleable handles serialization.
Run with the dependencies from rollup_spec/requirements.txt installed.
"""

import json
from pathlib import Path
import sys

from remerkleable.basic import uint64
from remerkleable.byte_arrays import ByteList
from remerkleable.complex import Container

ROOT = Path(__file__).resolve().parents[4]
sys.path.insert(0, str(ROOT / "rollup_spec/src"))
from rollup_spec import l2_execution_ssz, proof_io_v1, rollup_aggregation_ssz, rollup_ssz
from rollup_spec import stateless_input as ref
from rollup_spec.l1_rollup import FinalizationPublicInput, FinalizationSubmission

# The spec's reference l2-execution response: its public inputs are what the
# guest output frame encodes.
L2_EXECUTION_RESPONSE = ROOT / "rollup_spec/src/rollup_spec/prover_io/testdata/10-11-getZkL2ExecutionProofV1.response.json"
# The spec's reference rollup request.
ROLLUP_REQUEST = ROOT / "rollup_spec/src/rollup_spec/prover_io/testdata/10-14-getZkRollupProofV1.request.json"
# The spec's reference rollup response.
ROLLUP_RESPONSE = ROOT / "rollup_spec/src/rollup_spec/prover_io/testdata/10-14-getZkRollupProofV1.response.json"
# The spec's reference aggregation request.
AGGREGATION_REQUEST = ROOT / "rollup_spec/src/rollup_spec/prover_io/testdata/10-18-getZkRollupAggregationProofV1.request.json"
# The spec's reference aggregation response.
AGGREGATION_RESPONSE = ROOT / "rollup_spec/src/rollup_spec/prover_io/testdata/10-18-getZkRollupAggregationProofV1.response.json"


class EmptyExecutionRequests(Container):
    # Only empty lists are supported by this rollup. Element types and list
    # limits do not affect their serialization: five offsets, no list data.
    deposits: ByteList[1]
    withdrawals: ByteList[1]
    consolidations: ByteList[1]
    builder_deposits: ByteList[1]
    builder_exits: ByteList[1]


class NewPayloadRequest(ref.SszNewPayloadRequest):
    execution_requests: EmptyExecutionRequests


class StatelessInput(Container):
    # zesu's SszStatelessInput: payload and witness offsets, then the chain id
    # inline (a 16-byte fixed region). There is no public_keys field.
    new_payload_request: NewPayloadRequest
    witness: ref.SszExecutionWitness
    chain_config: uint64


def encode(obj):
    if obj["chainConfig"]["forkName"] != "Amsterdam":
        raise ValueError("only Amsterdam is supported")
    npr = obj["newPayloadRequest"]
    if any(npr["executionRequests"].values()):
        raise ValueError("execution requests must be empty")
    body = StatelessInput(
        new_payload_request=NewPayloadRequest(
            execution_payload=ref._ssz_execution_payload_from_obj(npr["executionPayload"]),
            versioned_hashes=[ref._hexbytes(h) for h in npr["versionedHashes"]],
            parent_beacon_block_root=ref._hexbytes(npr["parentBeaconBlockRoot"]),
            execution_requests=EmptyExecutionRequests(),
        ),
        witness=ref._ssz_execution_witness_from_obj(obj["executionWitness"]),
        chain_config=int(obj["chainConfig"]["chainId"]),
    )
    return bytes.fromhex("1501") + body.encode_bytes()


def l2_execution_output():
    # keccak256(frame) || frame, frame = 0x0003 || SSZ(public inputs).
    proof = proof_io_v1._decode_l2_execution_proof(json.loads(L2_EXECUTION_RESPONSE.read_text()), "").proof
    return l2_execution_ssz.encode_l2_execution_output(proof)


def rollup_inputs():
    # 0x0004 || SSZ(rollup private input), for the reference request and for a
    # variant with no parent blob data rolling hash and a calldata chunk.
    obj = json.loads(ROLLUP_REQUEST.read_text())
    reference = rollup_ssz.encode_rollup_input(proof_io_v1.decode_rollup_request(obj))
    request = obj["proofRequest"]
    request["parentSharedBlobCarryOverTailBytes"] = 0
    del request["parentBlobDataRollingHash"]
    request["chunks"][0].update(isCalldata=True, blobBytes="0x", calldataBytes="0xdead", kzgCommitment="0x", kzgProof="0x")
    calldata = rollup_ssz.encode_rollup_input(proof_io_v1.decode_rollup_request(obj))
    return reference, calldata


def rollup_request_guest_output():
    # The reference rollup request with each embedded l2-execution proof
    # carrying its guestOutput instead of its publicInputs.
    obj = json.loads(ROLLUP_REQUEST.read_text())
    request = proof_io_v1.decode_rollup_request(obj)
    for embedded, verifiable in zip(obj["proofRequest"]["l2ExecutionProofs"], request.l2_execution_proofs):
        del embedded["publicInputs"]
        embedded["guestOutput"] = "0x" + l2_execution_ssz.encode_l2_execution_output(verifiable.proof).hex()
    return obj


def rollup_output():
    # keccak256(frame) || frame, frame = 0x0005 || SSZ(public inputs).
    proof = proof_io_v1._decode_rollup_proof(json.loads(ROLLUP_RESPONSE.read_text()), "").proof
    return rollup_ssz.encode_rollup_output(proof)


def aggregation_inputs():
    # 0x0006 || SSZ(rollup-aggregation private input), for the reference request
    # and for a variant whose first rollup proof has messaging-block offsets.
    obj = json.loads(AGGREGATION_REQUEST.read_text())
    reference = rollup_aggregation_ssz.encode_aggregation_input(proof_io_v1.decode_aggregation_request(obj))
    obj["proofRequest"]["rollupProofs"][0]["publicInputs"]["l2MessagingBlocksOffsets"] = [1, 258]
    offsets = rollup_aggregation_ssz.encode_aggregation_input(proof_io_v1.decode_aggregation_request(obj))
    return reference, offsets


def aggregation_outputs():
    # keccak256(frame) || frame, frame = 0x0007 || SSZ(finalization public
    # inputs), for the public inputs of the reference response and for the
    # same public inputs with messaging-block offsets.
    pi = json.loads(AGGREGATION_RESPONSE.read_text())["publicInputs"]

    def output(pi):
        h = lambda v: bytes.fromhex(v[2:])
        public_inputs = FinalizationPublicInput(
            end_block_number=pi["endBlockNumber"],
            end_block_timestamp=pi["endBlockTimestamp"],
            end_l1_l2_bridge_rolling_hash=h(pi["endL1L2BridgeRollingHash"]),
            end_l1_l2_bridge_rolling_hash_message_number=pi["endL1L2BridgeRollingHashMessageNumber"],
            dynamic_chain_config_hash=h(pi["dynamicChainConfigHash"]),
            parent_ftx_rolling_hash=h(pi["parentFtxRollingHash"]),
            parent_ftx_number=pi["parentFtxNumber"],
            end_ftx_rolling_hash=h(pi["endFtxRollingHash"]),
            end_processed_ftx_number=pi["endProcessedFtxNumber"],
            parent_data_rolling_hash=h(pi["parentDataRollingHash"]),
            end_data_rolling_hash=h(pi["endDataRollingHash"]),
            parent_block_hash=h(pi["parentBlockHash"]),
            end_block_hash=h(pi["endBlockHash"]),
            parent_shared_blob_carry_over_tail_bytes=pi["parentSharedBlobCarryOverTailBytes"],
            final_shared_blob_carry_over_tail_bytes=pi["finalSharedBlobCarryOverTailBytes"],
            l2_l1_tree_depth=pi["l2L1TreeDepth"],
            l2_l1_roots=[h(v) for v in pi["l2L1Roots"]],
            filtered_addresses=[h(v) for v in pi["filteredAddresses"]],
            program_ids=[h(v) for v in pi["programIds"]],
            l2_messaging_blocks_offsets=pi["l2MessagingBlocksOffsets"],
        )
        return rollup_aggregation_ssz.encode_aggregation_output(
            FinalizationSubmission(public_inputs=public_inputs, proof=b"")
        )

    return output(pi), output({**pi, "l2MessagingBlocksOffsets": [1, 258]})


if __name__ == "__main__":
    directory = Path(__file__).resolve().parent
    for name in ("payload0", "payload1", "full"):
        data = encode(json.loads((directory / f"stateless_input_{name}.json").read_text()))
        (directory / f"stateless_input_{name}.ssz").write_bytes(data)
    (ROOT / "prover-ray/backend/jobadapter/testdata/single_block_expected.ssz").write_bytes(
        (directory / "stateless_input_payload0.ssz").read_bytes()
    )
    (directory / "l2_execution_output.ssz").write_bytes(l2_execution_output())
    reference, calldata = rollup_inputs()
    (directory / "rollup_input.ssz").write_bytes(reference)
    (directory / "rollup_input_calldata.ssz").write_bytes(calldata)
    (directory / "rollup_output.ssz").write_bytes(rollup_output())
    reference, offsets = aggregation_inputs()
    (directory / "aggregation_input.ssz").write_bytes(reference)
    (directory / "aggregation_input_offsets.ssz").write_bytes(offsets)
    reference, offsets = aggregation_outputs()
    (directory / "aggregation_output.ssz").write_bytes(reference)
    (directory / "aggregation_output_offsets.ssz").write_bytes(offsets)
    (ROOT / "prover-ray/backend/jobadapter/testdata/rollup_request_guest_output.json").write_text(
        json.dumps(rollup_request_guest_output(), indent=2) + "\n"
    )
