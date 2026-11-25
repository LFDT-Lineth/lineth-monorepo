"""Bench helpers for the rollup pipeline, with the spec as the reference.

request: write a getZkRollupProofV1 request from a rollup guest input
(0x0004 || SSZ), such as sample.request.ssz from the rollup guest's fixture tool.

check: check that a rollup response carries the public inputs of a rollup guest
output (hash || 0x0005 || SSZ), such as sample.output.bin.

Usage:
  python3 rollup_request.py request <input.ssz> <guest program id> <out dir>
  python3 rollup_request.py check <response.json> <output.bin>
"""

import json
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "rollup_spec/src"))
from rollup_spec import proof_io_v1, rollup_ssz


def hx(b):
    return "0x" + bytes(b).hex()


def request(input_path, guest_program_id, out_dir):
    data = Path(input_path).read_bytes()
    private_input = rollup_ssz.decode_rollup_input_ssz(data)

    proofs = []
    for p in private_input.l2_execution_proofs:
        # The embedded proof is the l2-execution response without proverVersion,
        # as the coordinator writes it.
        embedded = proof_io_v1.encode_response(p.proof, "", program_vk=p.program_vk)
        del embedded["proverVersion"]
        proofs.append(embedded)

    proof_request = {
        "chainId": int(private_input.chain_id),
        "conflations": [{"blockRlps": [hx(b) for b in c.block_rlps]} for c in private_input.conflations],
        "chunks": [
            {
                "chunkHash": hx(c.chunk_hash),
                "isCalldata": bool(c.is_calldata),
                "blobBytes": hx(c.blob_bytes),
                "calldataBytes": hx(c.calldata_bytes),
                "kzgCommitment": hx(c.kzg_commitment),
                "kzgProof": hx(c.kzg_proof),
            }
            for c in private_input.chunks
        ],
        "parentDataRollingHash": hx(private_input.parent_data_rolling_hash),
        "parentSharedBlobCarryOverTailBytes": int(private_input.parent_shared_blob_carry_over_tail_bytes),
        "l2ExecutionProofs": proofs,
    }
    if private_input.parent_blob_data_rolling_hash is not None:
        proof_request["parentBlobDataRollingHash"] = hx(private_input.parent_blob_data_rolling_hash)

    obj = {"guestProgramId": guest_program_id, "provingSystem": "bench", "proofRequest": proof_request}

    # The spec must read the request back to the same guest input.
    if rollup_ssz.encode_rollup_input(proof_io_v1.decode_rollup_request(obj)) != data:
        sys.exit("the request does not encode back to the input")

    start = proofs[0]["startBlockNumber"]
    end = proofs[-1]["publicInputs"]["endBlockNumber"]
    out = Path(out_dir) / f"{start}-{end}-getZkRollupProofV1.json"
    out.write_text(json.dumps(obj))
    print(out)


def check(response_path, output_path):
    response = json.loads(Path(response_path).read_text())
    output = Path(output_path).read_bytes()
    want = proof_io_v1._encode_rollup_public_inputs(rollup_ssz.decode_rollup_output_ssz(output))
    if response["publicInputs"] != want:
        for key in want:
            if response["publicInputs"].get(key) != want[key]:
                print(f"{key}: got {response['publicInputs'].get(key)}, want {want[key]}")
        sys.exit("the response public inputs differ from the guest output")
    print("the response public inputs equal the guest output")


if __name__ == "__main__":
    if len(sys.argv) == 5 and sys.argv[1] == "request":
        request(*sys.argv[2:])
    elif len(sys.argv) == 4 and sys.argv[1] == "check":
        check(*sys.argv[2:])
    else:
        sys.exit(__doc__)
