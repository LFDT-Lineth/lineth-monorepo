"""
The blob chunk of the `getZkRollupProofV1` request fixture, and its regeneration.

The chunk carries the segments of the request's two conflations behind a prefix the previous proof owns. The
conflations' canonical payloads are placeholders, as the fixture's block RLPs are. The chunk is built by the same
test-side compressor and blob builder as every other DA witness, so its frames are in the one form a DA frame takes.

From `rollup_spec/`, `PYTHONPATH=src python -m tests.rollup_request_fixture` rewrites the chunk's `chunkHash`,
`blobBytes`, `kzgCommitment` and `kzgProof`, and the request's `parentDataTailTakeBytes`, in the fixture.
"""

import json
from pathlib import Path

from tests.da_witness import blob_chunk, segment

FIXTURE = (
    Path(__file__).resolve().parents[1]
    / "src" / "rollup_spec" / "prover_io" / "testdata" / "10-14-getZkRollupProofV1.request.json"
)

FOREIGN_PREFIX = bytes.fromhex("abababab")
CANONICAL_PAYLOADS = (b"canonical payload", b"canonical payload")


def regenerated(request: dict) -> dict:
    """`request` with the blob chunk and the tail count rebuilt from the conflations' canonical payloads."""
    segments = b"".join(segment(payload) for payload in CANONICAL_PAYLOADS)
    chunk = blob_chunk(FOREIGN_PREFIX + segments)
    proof_request = request["proofRequest"]
    proof_request["parentDataTailTakeBytes"] = len(segments)
    proof_request["chunks"][0].update(
        chunkHash="0x" + chunk.chunk_hash.hex(),
        blobBytes="0x" + chunk.blob_bytes.hex(),
        kzgCommitment="0x" + chunk.kzg_commitment.hex(),
        kzgProof="0x" + chunk.kzg_proof.hex(),
    )
    return request


if __name__ == "__main__":
    FIXTURE.write_text(json.dumps(regenerated(json.loads(FIXTURE.read_text())), indent=2) + "\n")
