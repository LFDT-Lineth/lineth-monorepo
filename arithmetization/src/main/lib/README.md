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
| write_output        |   🟡   | custom-1 | 0b010  | 0b0000000 |
| ecrecover           |   🔴   | custom-1 |        |           |
| secp256k1_fp_mul    |   🟡   | custom-1 | 0b011  | 0b0000000 |
| secp256k1_fp_sq     |   🟡   | custom-1 | 0b011  | 0b0000001 |
| secp256k1_fp_inv    |   🟡   | custom-1 | 0b011  | 0b0000010 |
| secp256k1_fp_sqrt   |   🟡   | custom-1 | 0b011  | 0b0000011 |
| poly_eval           |   🔴   | custom-1 |        |           |
| ...                 |   🔴   | custom-1 |        |           |
|---------------------|--------|----------|--------|-----------|

secp256k1 field accelerator (🟡 = opt-in): `rd` = destination pointer, `rs1` = a, `rs2` = b (ignored except by `mul`). `inv` returns 0 for 0. `sqrt` writes 40 bytes: r = a^((p+1)/4) and a u64 flag at +32 (1 = a is a square; 0 = non-square, then r² = -a). inv and sqrt are hinted (`expmod_unconstrained_256`) and checked with one `mulmod_256`.
Each operand is 32 bytes, 8-byte aligned, little-endian (four u64). Inputs may be any 256-bit value; the
result is canonical (< p). Each op lives in `lib/secp256k1_fp/{mul,inv,sqrt}.zkc` with a `*_stub.zkc` twin. `riscv/main_fieldaccel.zkc` compiles in all; `main_fieldaccel_{mul,inv,sqrt}.zkc` compile in one, to measure each separately. Copy one to combine others (it uses the native `mulmod_256`, so `zkc exec --gogen` cannot run it).
`riscv/main.zkc` links the stubs, which fails if the instruction is issued.
