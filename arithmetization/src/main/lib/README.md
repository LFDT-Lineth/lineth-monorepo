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

|-------------------|--------|----------|--------|-----------|
| Other precompiles | status | opc      | funct3 | funct7    |
|-------------------|:------:|----------|--------|-----------|
| keccak            |   🟢   | custom-1 | 0b000  | 0b0000000 |
| poseidon1         |   🟡   | custom-1 | 0b111  | 0b1111111 |
| ecrecover         |   🔴   | custom-1 |        |           |
| poly_eval         |   🔴   | custom-1 |        |           |
| ...               |   🔴   | custom-1 |        |           |
|-------------------|--------|----------|--------|-----------|
