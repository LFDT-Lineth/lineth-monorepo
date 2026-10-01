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
from rollup_spec import stateless_input as ref


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


class StatelessInput(ref.SszStatelessInput):
    chain_config: uint64
    new_payload_request: NewPayloadRequest


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
        public_keys=ref._recover_public_keys(
            npr["executionPayload"]["transactions"], int(obj["chainConfig"]["chainId"])
        ),
    )
    return bytes.fromhex("1501") + body.encode_bytes()


if __name__ == "__main__":
    directory = Path(__file__).resolve().parent
    for name in ("payload0", "payload1", "full"):
        data = encode(json.loads((directory / f"stateless_input_{name}.json").read_text()))
        (directory / f"stateless_input_{name}.ssz").write_bytes(data)
    (ROOT / "prover-ray/backend/jobadapter/testdata/single_block_expected.ssz").write_bytes(
        (directory / "stateless_input_payload0.ssz").read_bytes()
    )
