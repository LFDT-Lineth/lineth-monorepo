"""Bench helpers for the aggregation pipeline, with the spec as the reference.

request: write a getZkRollupAggregationProofV1 request from a rollup-aggregation
guest input (0x0006 || SSZ), such as the guest's test/testdata/sample.request.ssz.
Each embedded rollup proof is a rollup response without proverVersion, as the
coordinator writes it, with its public inputs as publicInputs or as guestOutput.

check: check that an aggregation response carries the public inputs of a
rollup-aggregation guest output (hash || 0x0007 || SSZ), such as
sample.output.bin, and a BN254 PLONK proof in the Solidity layout.

Usage:
  python3 aggregation_request.py request <input.ssz> <guest program id> <publicInputs|guestOutput> <out dir>
  python3 aggregation_request.py check <response.json> <output.bin>
"""

import json
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "rollup_spec/src"))
from rollup_spec import proof_io_v1, rollup_aggregation_ssz, rollup_ssz

# A BN254 PLONK proof with no BSB22 commitment in the gnark Solidity layout.
SOLIDITY_PLONK_PROOF_SIZE = 768


def hx(b):
    return "0x" + bytes(b).hex()


def request(input_path, guest_program_id, form, out_dir):
    if form not in ("publicInputs", "guestOutput"):
        sys.exit(f"unknown format {form}: use publicInputs or guestOutput")
    data = Path(input_path).read_bytes()
    private_input = rollup_aggregation_ssz.decode_aggregation_input_ssz(data)

    proofs = []
    for v in private_input.rollup_proofs:
        embedded = proof_io_v1.encode_rollup_response(v.proof, "", program_vk=v.program_vk)
        del embedded["proverVersion"]
        if form == "guestOutput":
            del embedded["publicInputs"]
            embedded["guestOutput"] = hx(rollup_ssz.encode_rollup_output(v.proof))
        proofs.append(embedded)

    obj = {"guestProgramId": guest_program_id, "provingSystem": "bench", "proofRequest": {"rollupProofs": proofs}}

    # The spec must read a publicInputs request back to the same guest input.
    if form == "publicInputs":
        if rollup_aggregation_ssz.encode_aggregation_input(proof_io_v1.decode_aggregation_request(obj)) != data:
            sys.exit("the request does not encode back to the input")

    start = int(private_input.rollup_proofs[0].proof.start_block_number)
    end = int(private_input.rollup_proofs[-1].proof.public_inputs.end_block_number)
    out = Path(out_dir) / f"{start}-{end}-getZkRollupAggregationProofV1.json"
    out.write_text(json.dumps(obj))
    print(out)


def check(response_path, output_path):
    response = json.loads(Path(response_path).read_text())
    output = Path(output_path).read_bytes()
    submission = rollup_aggregation_ssz.decode_aggregation_output_ssz(output)
    want = proof_io_v1.encode_aggregation_response(submission, "", start_block_number=0)["publicInputs"]
    failed = False
    if response["publicInputs"] != want:
        for key in want:
            if response["publicInputs"].get(key) != want[key]:
                print(f"{key}: got {response['publicInputs'].get(key)}, want {want[key]}")
        failed = True
    proof = bytes.fromhex(response["proof"][2:])
    if len(proof) != SOLIDITY_PLONK_PROOF_SIZE:
        print(f"proof: {len(proof)} bytes, want {SOLIDITY_PLONK_PROOF_SIZE} (a BN254 PLONK proof)")
        failed = True
    if failed:
        sys.exit("the response does not match the guest output")
    print("the response public inputs equal the guest output, and the proof is a BN254 PLONK proof")


if __name__ == "__main__":
    if len(sys.argv) == 6 and sys.argv[1] == "request":
        request(*sys.argv[2:])
    elif len(sys.argv) == 4 and sys.argv[1] == "check":
        check(*sys.argv[2:])
    else:
        sys.exit(__doc__)
