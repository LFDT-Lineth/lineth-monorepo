# SSZ vectors

`EncodeStatelessInput` targets the zesu decoder
(`src/stateless/stateless/ssz.zig`) at the commit pinned by
`riscv-guests/l2_execution/build.zig.zon`:

- Inner schema `0x1501`: Amsterdam fork index `0x15`, revision `1`.
- A 16-byte fixed section: payload offset, witness offset, inline uint64 chain ID.
  There is no public-keys field.
- Witness items are bounded per item: 1 KiB per state node and header, 64 KiB
  per code.
- Five empty execution-request lists: deposits, withdrawals, consolidations,
  builder deposits and builder exits.
- The outer L2 execution envelope uses `0x0002`.

`generate.py` defines the wire containers and uses `rollup_spec` field converters
with remerkleable for serialization. It supports empty execution requests only.

From the repository root, in an environment with `rollup_spec/requirements.txt`
installed (Python 3.11 or 3.12):

```bash
python prover-ray/utils/ssz/testdata/generate.py
cd prover-ray && go test ./utils/ssz ./backend ./backend/jobadapter
```

The generator also updates
`backend/jobadapter/testdata/single_block_expected.ssz`, which duplicates payload0,
and writes `l2_execution_output.ssz`: the `guestOutput` of the spec's reference
response
`rollup_spec/src/rollup_spec/prover_io/testdata/10-11-getZkL2ExecutionProofV1.response.json`
as raw bytes, for `riscv-guests/guest_common/test/l2_execution_proof_test.zig`.

It also writes the rollup guest input (`0x0004 || SSZ(rollup private input)`) for
the spec's reference request
`rollup_spec/src/rollup_spec/prover_io/testdata/10-14-getZkRollupProofV1.request.json`
(`rollup_input`), and for the same request with no boundary hash and a calldata
chunk in place of the blob (`rollup_input_calldata`). `backend/jobadapter` builds
the input from each request and checks it against these bytes.

| Fixture | Bytes | SHA-256 |
| --- | ---: | --- |
| payload0 | 772 | `1c5f4c6d7824e006abfbc13bb5b766b78a7fc61efe2d1bb3e65912b274d2fe3e` |
| payload1 | 772 | `88d3c88127dd4460668fc9c83a3098b97649aa6888326d55666e8ca392600642` |
| full | 1099 | `ad56803912126e7d60ed5eee3f89b742ec6908c5fc3f0daef93e50db51cceaaa` |
| l2_execution_output | 370 | `d16135f2d4662a0cd7cf30a492aff3718602a549948684cb47e3e2eeb361b293` |
| rollup_input | 132197 | `bc47b47474e44b7825d969da5010b89d49920ff1592ec070ee4511421d7e3cdc` |
| rollup_input_calldata | 999 | `2459d8aeb97ea9bec83f2a807f4d4b99ca5364985128dd57bf2de608b9c00a88` |

`payload0` and `payload1` cover single-block coordinator fixture inputs.
`full` additionally covers withdrawals, versioned hashes, extra data, block access
lists, a nonzero slot number, a multibyte base fee, both legacy and EIP-1559
transactions, and multiple witness entries. These synthetic fixtures test
serialization, not valid block execution.

## Executable guest fixture

`backend/jobadapter/testdata/request_guest_fixture.json` is the JSON form of
`riscv-guests/l2_execution/test/testdata/stateless_input.ssz`, including its real
witness data.
`TestDecodeL2ExecutionRequest_Regression_GuestFixture` checks that the entire
JSON-to-SSZ path reproduces the guest fixture byte for byte, independently of the
Python-generated golden vectors.
