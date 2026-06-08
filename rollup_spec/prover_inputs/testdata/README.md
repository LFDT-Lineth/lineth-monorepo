# V1 prover I/O test fixtures

Fully-valid `getZkL2ExecutionProofV1` request/response payloads (real hex, no
ellipses) used by `rollup_spec/test_proof_io_v1.py`.

These differ from the illustrative examples in `../` (the parent
`prover_inputs/` directory): those use `0x...` placeholders for documentation and
therefore do **not** validate against the JSON Schemas in `rollup_spec/schemas/`.
The fixtures here do validate, and the request/response pair is mutually
consistent so the codec round-trip (`decode_request`, `encode_response`) can be
asserted byte-for-byte.
