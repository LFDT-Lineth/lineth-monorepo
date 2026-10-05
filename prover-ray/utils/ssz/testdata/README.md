# SSZ vectors

`EncodeStatelessInput` targets the decoder in zesu
[`b5483dda21d72405c50a22cb6406a1231e51a579`](https://github.com/Consensys-Incorporated/zesu/blob/b5483dda21d72405c50a22cb6406a1231e51a579/src/stateless/stateless/ssz.zig),
pinned by `riscv-guests/l2-execution/build.zig.zon`:

- Inner schema `0x1501`: Amsterdam fork index `0x15`, revision `1`.
- A 20-byte fixed section: payload offset, witness offset, inline uint64 chain ID,
  public-keys offset.
- Five empty execution-request lists: deposits, withdrawals, consolidations,
  builder deposits and builder exits.
- The outer L2 execution envelope uses `0x0002`.

`generate.py` defines the wire containers and uses `rollup_spec` field converters
with remerkleable for serialization. It supports empty execution requests only.

From the repository root, in an environment with `rollup_spec/requirements.txt`
installed (Python 3.11 or 3.12):

```bash
python prover-ray/utils/ssz/testdata/generate.py
cd prover-ray && go test ./utils/ssz ./backend/jobadapter
```

The generator also updates
`backend/jobadapter/testdata/single_block_expected.ssz`, which duplicates payload0.

| Fixture | Bytes | SHA-256 |
| --- | ---: | --- |
| payload0 | 841 | `60d903488c8a3a73aa267930359e1f0f48536e67cdcd2b139c1f9207dca792b8` |
| payload1 | 841 | `1b4bb56d8f189883f3078224e54601a2a5ff63c7da4d76c95b1bf5b0985d966f` |
| full | 1233 | `82a85fc53decc493859c4e1418915e0d49e52747baef669ab8d1b63bd673331d` |

`payload0` and `payload1` cover single-block coordinator fixture inputs.
`full` additionally covers withdrawals, versioned hashes, extra data, block access
lists, a nonzero slot number, a multibyte base fee, both legacy and EIP-1559
transactions, and multiple witness entries. These synthetic fixtures test
serialization, not valid block execution.

## Executable guest fixture

`backend/jobadapter/testdata/request_guest_fixture.json` is the JSON form of
`riscv-guests/l2-execution/test/testdata/stateless_input.ssz`, including its real
witness data.
`TestDecodeL2ExecutionRequest_Regression_GuestFixture` checks that the entire
JSON-to-SSZ path reproduces the guest fixture byte for byte, independently of the
Python-generated golden vectors.
