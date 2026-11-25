This folder contains the zkvm library: zkc implementations of EVM precompiles and of various other accelerants.

|---------------------|--------|----------|--------|-----------|
| EVM precompiles     | status |    opc   | funct3 |   funct7  |
|---------------------|:------:|:--------:|:------:|:---------:|
| ECRECOVER           |   🔴   | custom-0 |  0b... | 0b......1 |
| SHA2-256            |   🔴   | custom-0 |  0b... | 0b.....10 |
| RIPEMD              |   🔴   | custom-0 |  0b... | 0b.....11 |
| IDENTITY            |   🔴   | custom-0 |  0b... | 0b....100 |
| MODEXP_small        |   🔴   | custom-0 |  0b..0 | 0b....101 |
| MODEXP_large        |   🔴   | custom-0 |  0b..1 | 0b....101 |
| ECADD               |   🔴   | custom-0 |  0b... | 0b....110 |
| ECMUL               |   🔴   | custom-0 |  0b... | 0b....111 |
| ECPAIRING           |   🔴   | custom-0 |  0b... | 0b...1000 |
| BLAKE2f             |   🔴   | custom-0 |  0b... | 0b...1001 |
| POINTEVALUATION     |   🔴   | custom-0 |  0b... | 0b...1010 |
| BLS12_G1ADD         |   🔴   | custom-0 |  0b... | 0b...1011 |
| BLS12_G1MSM         |   🔴   | custom-0 |  0b... | 0b...1100 |
| BLS12_G2ADD         |   🔴   | custom-0 |  0b... | 0b...1101 |
| BLS12_G2MSM         |   🔴   | custom-0 |  0b... | 0b...1110 |
| BLS12_PAIRING_CHECK |   🔴   | custom-0 |  0b... | 0b...1111 |
| BLS12_MAP_FP_TO_G1  |   🔴   | custom-0 |  0b... | 0b..10000 |
| BLS12_MAP_FP2_TO_G2 |   🔴   | custom-0 |  0b... | 0b..10001 |
| P256_VERIFY         |   🔴   | custom-0 |  0b..1 | 0b....... |
|---------------------|--------|----------|--------|-----------|

Note. We use '.' to represent '0'.

|---------------------|--------|----------|--------|-----------|
| Other 'precompiles' | status | opc      | funct3 | funct7    |
|---------------------|:------:|----------|--------|-----------|
| keccak              |   🟢   | custom-1 | 0b000  | 0b0000000 |
| poseidon2           |   🟢   | custom-1 | 0b001  | 0b0000000 |
| ecrecover           |   🔴   | custom-1 |        |           |
| poly_eval           |   🔴   | custom-1 |        |           |
| ...                 |   🔴   | custom-1 |        |           |
|---------------------|--------|----------|--------|-----------|

The field accelerator is an ECALL, not a custom opcode, so the predecoder needs no change for it.
`field/field.zkc` implements it.

|---------------------|--------|-------------------------------------------------------------|
| ECALL               | status | registers                                                   |
|---------------------|:------:|-------------------------------------------------------------|
| field op            |   🟡   | a7 = 0xF0, a0 = op, a1 = field, a2 = out, a3 = a, a4 = b    |
|---------------------|--------|-------------------------------------------------------------|

- Ops (in `field/field.zkc`): `FIELD_OP_MUL` (out = a·b), `FIELD_OP_DIV` (out = a/b, or 0 and status 1 if b = 0), and
  `FIELD_OP_SQRT` (out² = a, or out² = −a and status 1 if a is not a square). An inverse is
  DIV with a = 1.
- Fields: `FIELD_MODULUS` ids `1..7` (256-bit: secp256k1, secp256r1, BN254 Fp/Fr, and BLS12-381 Fr)
  and `8` (384-bit: BLS12-381 Fp). SQRT needs p = 3 (mod 4) (secp256k1, secp256r1, BN254, and
  BLS12-381 Fp).
- Operands are four (`1..7`, 32 bytes) or six (`8`, 48 bytes) little-endian u64 limbs below 2p
  (not checked: a larger one gives an unspecified result); any 256-bit value qualifies when
  p > 2^255. b is read for every op. The output is reduced and may alias an input. The status comes
  back in a0.
- Each call makes one constrained `mulmod_384` (prover-ray's NonNative gadget). All three ops
  take their output witness from the unconstrained `hint_field_op_384`, which computes modular
  products in 768-bit VM arithmetic without adding `mulmod_384` rows.
- The guest-side C interfaces are `lineth_zkvm_field_op_256` and `lineth_zkvm_field_op_384` in
  `riscv-guests/lineth_accelerators/include/lineth_accelerators.h` (both wrappers invoke ECALL `0xF0`).

Guest output is not a precompile: `write_output` issues the Linux `write` syscall (`a7=64`) with
fd 4 (`GUEST_OUT`, public output) or fd 3 (`GUEST_LOG`, private log). See `process_syscall` in
`../riscv/interpreter.zkc`.
