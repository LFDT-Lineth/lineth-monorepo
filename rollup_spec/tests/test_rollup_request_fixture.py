"""The blob chunk of the `getZkRollupProofV1` request fixture is the regenerated one and valid DA material."""

import json

from ethereum.crypto.hash import keccak256

from rollup_spec.rollup import ChunkWitness, verify_da_chunks
from tests.da_witness import data_rolling_hash_after
from tests.rollup_request_fixture import CANONICAL_PAYLOADS, FIXTURE, regenerated


def test_blob_chunk_is_the_regenerated_one():
    request = json.loads(FIXTURE.read_text())

    assert regenerated(json.loads(FIXTURE.read_text())) == request


def test_blob_chunk_is_a_valid_stream_of_the_canonical_payloads():
    proof_request = json.loads(FIXTURE.read_text())["proofRequest"]
    fields = proof_request["chunks"][0]
    chunk = ChunkWitness(
        bytes.fromhex(fields["chunkHash"][2:]),
        is_calldata=False,
        blob_bytes=bytes.fromhex(fields["blobBytes"][2:]),
        kzg_commitment=bytes.fromhex(fields["kzgCommitment"][2:]),
        kzg_proof=bytes.fromhex(fields["kzgProof"][2:]),
    )
    boundary = keccak256(b"dataRollingHash before the blob")
    parent = data_rolling_hash_after(boundary, [chunk])

    end_hash, discard = verify_da_chunks(
        [chunk], list(CANONICAL_PAYLOADS), parent, proof_request["parentDataTailTakeBytes"], boundary,
    )

    assert (end_hash, discard) == (parent, 0)
