#!/usr/bin/env python3
"""
Generate the checked-in `.ssz` golden vectors for the rollup and
rollup-aggregation guest programs, one per JSON fixture under
`prover_io/testdata/`.

Request fixtures decode straight through `proof_io_v1` into the guest INPUT
dataclass, then `rollup_ssz` encodes the framed SSZ bytes a guest would read.
Response fixtures carry a full prover response (`proverVersion` + `proof` +
guest output); only the guest-emitted OUTPUT fields are meaningful on the SSZ
wire (a guest cannot attest its own proof), so this script drops
`proverVersion`/`proof` and encodes the remaining fields with
`encode_rollup_output` / `encode_aggregation_output`.

Each output file is the sibling of its JSON fixture, same basename with a
`.ssz` extension, so `test_rollup_ssz.py` can pair them up by glob.

Run from the `rollup_spec/` directory:  .venv/bin/python scripts/generate_rollup_ssz_golden_vectors.py
"""

import json
from pathlib import Path

import rollup_spec
from rollup_spec.proof_io_v1 import (
    _decode_rollup_public_input,
    decode_aggregation_request,
    decode_rollup_request,
)
from rollup_spec.rollup import RollupProof
from rollup_spec.rollup_ssz import (
    encode_aggregation_input,
    encode_aggregation_output,
    encode_rollup_input,
    encode_rollup_output,
)
from ethereum.crypto.hash import Hash32
from ethereum.state import Address
from ethereum_types.numeric import U64
from rollup_spec.l1_rollup import FinalizationSubmission

_TESTDATA_DIR = Path(rollup_spec.__file__).resolve().parent / "prover_io" / "testdata"


def _hexbytes(value: str) -> bytes:
    return bytes.fromhex(value[2:] if value[:2] in ("0x", "0X") else value)


def _load(path: Path) -> dict:
    return json.loads(path.read_text())


def _write_ssz(json_path: Path, data: bytes) -> None:
    ssz_path = json_path.with_suffix(".ssz")
    ssz_path.write_bytes(data)
    print(f"wrote {ssz_path.relative_to(_TESTDATA_DIR.parent.parent)} ({len(data)} bytes)")


def _rollup_output_from_response(resp: dict) -> RollupProof:
    pi = _decode_rollup_public_input(resp["publicInputs"], "publicInputs.")
    return RollupProof(
        public_inputs=pi,
        start_block_number=U64(resp["startBlockNumber"]),
        l2_l1_roots=[Hash32(_hexbytes(h)) for h in resp["l2L1Roots"]],
        filtered_addresses=[Address(_hexbytes(a)) for a in resp["filteredAddresses"]],
    )


def _aggregation_output_from_response(resp: dict) -> FinalizationSubmission:
    pi = _decode_rollup_public_input(resp["publicInputs"], "publicInputs.")
    return FinalizationSubmission(
        public_inputs=pi,
        proof=b"",
        l2_l1_roots=[Hash32(_hexbytes(h)) for h in resp["l2L1Roots"]],
        filtered_addresses=[Address(_hexbytes(a)) for a in resp["filteredAddresses"]],
        l2_messaging_blocks_offsets=list(resp["l2MessagingBlocksOffsets"]),
    )


def main() -> None:
    rollup_request_path = next(_TESTDATA_DIR.glob("*getZkRollupProofV1.request.json"))
    rollup_response_path = next(_TESTDATA_DIR.glob("*getZkRollupProofV1.response.json"))
    aggregation_request_path = next(
        _TESTDATA_DIR.glob("*getZkRollupAggregationProofV1.request.json")
    )
    aggregation_response_path = next(
        _TESTDATA_DIR.glob("*getZkRollupAggregationProofV1.response.json")
    )

    rollup_input = decode_rollup_request(_load(rollup_request_path))
    _write_ssz(rollup_request_path, encode_rollup_input(rollup_input))

    rollup_output = _rollup_output_from_response(_load(rollup_response_path))
    _write_ssz(rollup_response_path, encode_rollup_output(rollup_output))

    aggregation_input = decode_aggregation_request(_load(aggregation_request_path))
    _write_ssz(aggregation_request_path, encode_aggregation_input(aggregation_input))

    aggregation_output = _aggregation_output_from_response(_load(aggregation_response_path))
    _write_ssz(aggregation_response_path, encode_aggregation_output(aggregation_output))


if __name__ == "__main__":
    main()
