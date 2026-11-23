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
from rollup_spec import l2_execution_ssz, proof_io_v1, rollup_ssz
from rollup_spec import stateless_input as ref

# The spec's reference l2-execution response: its public inputs are what the
# guest output frame encodes.
L2_EXECUTION_RESPONSE = ROOT / "rollup_spec/src/rollup_spec/prover_io/testdata/10-11-getZkL2ExecutionProofV1.response.json"
# The spec's reference rollup request.
ROLLUP_REQUEST = ROOT / "rollup_spec/src/rollup_spec/prover_io/testdata/10-14-getZkRollupProofV1.request.json"
# The spec's reference rollup response: its public inputs are what the rollup
# guest output frame encodes.
ROLLUP_RESPONSE = ROOT / "rollup_spec/src/rollup_spec/prover_io/testdata/10-14-getZkRollupProofV1.response.json"


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


def rollup_output():
    # keccak256(frame) || frame, frame = 0x0005 || SSZ(public inputs).
    proof = proof_io_v1._decode_rollup_proof(json.loads(ROLLUP_RESPONSE.read_text()), "").proof
    return rollup_ssz.encode_rollup_output(proof)


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
