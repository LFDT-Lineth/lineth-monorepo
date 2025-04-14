(defconst
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                  ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; EVM INSTRUCTIONS ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                  ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;; Stop and Arithmetic Operations
  EVM_INST_STOP                             0x00
  EVM_INST_ADD                              0x01
  EVM_INST_MUL                              0x02
  EVM_INST_SUB                              0x03
  EVM_INST_DIV                              0x04
  EVM_INST_SDIV                             0x05
  EVM_INST_MOD                              0x06
  EVM_INST_SMOD                             0x07
  EVM_INST_ADDMOD                           0x08
  EVM_INST_MULMOD                           0x09
  EVM_INST_EXP                              0x0a
  EVM_INST_SIGNEXTEND                       0x0b
  ;; Comparison & Bitwise Logic Operations
  EVM_INST_LT                               0x10
  EVM_INST_GT                               0x11
  EVM_INST_SLT                              0x12
  EVM_INST_SGT                              0x13
  EVM_INST_EQ                               0x14
  EVM_INST_ISZERO                           0x15
  EVM_INST_AND                              0x16
  EVM_INST_OR                               0x17
  EVM_INST_XOR                              0x18
  EVM_INST_NOT                              0x19
  EVM_INST_BYTE                             0x1a
  EVM_INST_SHL                              0x1b
  EVM_INST_SHR                              0x1c
  EVM_INST_SAR                              0x1d
  ;; KECCAK256
  EVM_INST_SHA3                             0x20
  ;; Environmental Information
  EVM_INST_ADDRESS                          0x30
  EVM_INST_BALANCE                          0x31
  EVM_INST_ORIGIN                           0x32
  EVM_INST_CALLER                           0x33
  EVM_INST_CALLVALUE                        0x34
  EVM_INST_CALLDATALOAD                     0x35
  EVM_INST_CALLDATASIZE                     0x36
  EVM_INST_CALLDATACOPY                     0x37
  EVM_INST_CODESIZE                         0x38
  EVM_INST_CODECOPY                         0x39
  EVM_INST_GASPRICE                         0x3A
  EVM_INST_EXTCODESIZE                      0x3B
  EVM_INST_EXTCODECOPY                      0x3C
  EVM_INST_RETURNDATASIZE                   0x3D
  EVM_INST_RETURNDATACOPY                   0x3E
  EVM_INST_EXTCODEHASH                      0x3F
  ;; Block Information
  EVM_INST_BLOCKHASH                        0x40
  EVM_INST_COINBASE                         0x41
  EVM_INST_TIMESTAMP                        0x42
  EVM_INST_NUMBER                           0x43
  EVM_INST_DIFFICULTY                       0x44                               ;; PREVRANDAO comes after London, in Paris
  EVM_INST_GASLIMIT                         0x45
  EVM_INST_CHAINID                          0x46
  EVM_INST_SELFBALANCE                      0x47
  EVM_INST_BASEFEE                          0x48
  ;; Stack, Memory, Storage and Flow Operations
  EVM_INST_POP                              0x50
  EVM_INST_MLOAD                            0x51
  EVM_INST_MSTORE                           0x52
  EVM_INST_MSTORE8                          0x53
  EVM_INST_SLOAD                            0x54
  EVM_INST_SSTORE                           0x55
  EVM_INST_JUMP                             0x56
  EVM_INST_JUMPI                            0x57
  EVM_INST_PC                               0x58
  EVM_INST_MSIZE                            0x59
  EVM_INST_GAS                              0x5A
  EVM_INST_JUMPDEST                         0x5B
  ;; Push Operations
  EVM_INST_PUSH0                            0x5F ;; post Shanghai
  EVM_INST_PUSH1                            0x60
  EVM_INST_PUSH2                            0x61
  EVM_INST_PUSH3                            0x62
  EVM_INST_PUSH4                            0x63
  EVM_INST_PUSH5                            0x64
  EVM_INST_PUSH6                            0x65
  EVM_INST_PUSH7                            0x66
  EVM_INST_PUSH8                            0x67
  EVM_INST_PUSH9                            0x68
  EVM_INST_PUSH10                           0x69
  EVM_INST_PUSH11                           0x6A
  EVM_INST_PUSH12                           0x6B
  EVM_INST_PUSH13                           0x6C
  EVM_INST_PUSH14                           0x6D
  EVM_INST_PUSH15                           0x6E
  EVM_INST_PUSH16                           0x6F
  EVM_INST_PUSH17                           0x70
  EVM_INST_PUSH18                           0x71
  EVM_INST_PUSH19                           0x72
  EVM_INST_PUSH20                           0x73
  EVM_INST_PUSH21                           0x74
  EVM_INST_PUSH22                           0x75
  EVM_INST_PUSH23                           0x76
  EVM_INST_PUSH24                           0x77
  EVM_INST_PUSH25                           0x78
  EVM_INST_PUSH26                           0x79
  EVM_INST_PUSH27                           0x7A
  EVM_INST_PUSH28                           0x7B
  EVM_INST_PUSH29                           0x7C
  EVM_INST_PUSH30                           0x7D
  EVM_INST_PUSH31                           0x7E
  EVM_INST_PUSH32                           0x7F
  ;; Duplication Operations
  EVM_INST_DUP1                             0x80
  EVM_INST_DUP2                             0x81
  EVM_INST_DUP3                             0x82
  EVM_INST_DUP4                             0x83
  EVM_INST_DUP5                             0x84
  EVM_INST_DUP6                             0x85
  EVM_INST_DUP7                             0x86
  EVM_INST_DUP8                             0x87
  EVM_INST_DUP9                             0x88
  EVM_INST_DUP10                            0x89
  EVM_INST_DUP11                            0x8A
  EVM_INST_DUP12                            0x8B
  EVM_INST_DUP13                            0x8C
  EVM_INST_DUP14                            0x8D
  EVM_INST_DUP15                            0x8E
  EVM_INST_DUP16                            0x8F
  ;; Exchange Operations
  EVM_INST_SWAP1                            0x90
  EVM_INST_SWAP2                            0x91
  EVM_INST_SWAP3                            0x92
  EVM_INST_SWAP4                            0x93
  EVM_INST_SWAP5                            0x94
  EVM_INST_SWAP6                            0x95
  EVM_INST_SWAP7                            0x96
  EVM_INST_SWAP8                            0x97
  EVM_INST_SWAP9                            0x98
  EVM_INST_SWAP10                           0x99
  EVM_INST_SWAP11                           0x9A
  EVM_INST_SWAP12                           0x9B
  EVM_INST_SWAP13                           0x9C
  EVM_INST_SWAP14                           0x9D
  EVM_INST_SWAP15                           0x9E
  EVM_INST_SWAP16                           0x9F
  ;; Logging Operations
  EVM_INST_LOG0                             0xA0
  EVM_INST_LOG1                             0xA1
  EVM_INST_LOG2                             0xA2
  EVM_INST_LOG3                             0xA3
  EVM_INST_LOG4                             0xA4
  ;; System Operations
  EVM_INST_CREATE                           0xF0
  EVM_INST_CALL                             0xF1
  EVM_INST_CALLCODE                         0xF2
  EVM_INST_RETURN                           0xF3
  EVM_INST_DELEGATECALL                     0xF4
  EVM_INST_CREATE2                          0xF5
  EVM_INST_STATICCALL                       0xFA
  EVM_INST_REVERT                           0xFD
  EVM_INST_INVALID                          0xFE
  EVM_INST_SELFDESTRUCT                     0xFF
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;               ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; GAS CONSTANTS ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;               ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  GAS_CONST_G_ZERO                          0
  GAS_CONST_G_JUMPDEST                      1
  GAS_CONST_G_BASE                          2
  GAS_CONST_G_VERY_LOW                      3
  GAS_CONST_G_LOW                           5
  GAS_CONST_G_MID                           8
  GAS_CONST_G_HIGH                          10
  GAS_CONST_G_WARM_ACCESS                   100
  GAS_CONST_G_ACCESS_LIST_ADRESS            2400
  GAS_CONST_G_ACCESS_LIST_STORAGE           1900
  GAS_CONST_G_COLD_ACCOUNT_ACCESS           2600
  GAS_CONST_G_COLD_SLOAD                    2100
  GAS_CONST_G_SSET                          20000
  GAS_CONST_G_SRESET                        2900
  REFUND_CONST_R_SCLEAR                     4800
  GAS_CONST_G_SELFDESTRUCT                  5000
  GAS_CONST_G_CREATE                        32000
  GAS_CONST_G_CODE_DEPOSIT                  200
  GAS_CONST_G_CALL_VALUE                    9000
  GAS_CONST_G_CALL_STIPEND                  2300
  GAS_CONST_G_NEW_ACCOUNT                   25000
  GAS_CONST_G_EXP                           10
  GAS_CONST_G_EXP_BYTE                      50
  GAS_CONST_G_MEMORY                        3
  GAS_CONST_G_TX_CREATE                     32000
  GAS_CONST_G_TX_DATA_ZERO                  4
  GAS_CONST_G_TX_DATA_NONZERO               16
  GAS_CONST_G_TRANSACTION                   21000
  GAS_CONST_G_LOG                           375
  GAS_CONST_G_LOG_DATA                      8
  GAS_CONST_G_LOG_TOPIC                     375
  GAS_CONST_G_KECCAK_256                    30
  GAS_CONST_G_KECCAK_256_WORD               6
  GAS_CONST_G_COPY                          3
  GAS_CONST_G_BLOCKHASH                     20
  GAS_CONST_ECRECOVER                       3000
  GAS_CONST_SHA2                            60
  GAS_CONST_SHA2_WORD                       12
  GAS_CONST_RIPEMD                          600
  GAS_CONST_RIPEMD_WORD                     120
  GAS_CONST_IDENTITY                        15
  GAS_CONST_IDENTITY_WORD                   3
  GAS_CONST_MODEXP                          200 
  GAS_CONST_ECADD                           150
  GAS_CONST_ECMUL                           6000
  GAS_CONST_ECPAIRING                       45000
  GAS_CONST_ECPAIRING_PAIR                  34000
  GAS_CONST_BLAKE2_PER_ROUND                1
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;           ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;  EVM MISC ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;           ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  MAX_CODE_SIZE                             24576
  EIP_3541_MARKER                           0xEF
  BLOCKHASH_MAX_HISTORY                     256
  MAX_REFUND_QUOTIENT                       5
  CREATE2_SHIFT                             0xff                               ;; create2 first byte
  EIP2681_MAX_NONCE                         18446744073709551615               ;; = 2^64 - 1
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;             ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;  LINEA MISC ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;             ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  LINEA_CHAIN_ID                            0xe708
  LINEA_GOERLI_CHAIN_ID                     0xe704
  LINEA_SEPOLIA_CHAIN_ID                    0xe705
  LINEA_BASE_FEE                            7
  LINEA_DIFFICULTY                          2
  LINEA_MAX_NUMBER_OF_TRANSACTIONS_IN_BATCH 200
  GAS_LIMIT_ADJUSTMENT_FACTOR               1024
  ;; we keep the following constants as they are referenced in blockdata gas limit tests
  ETHEREUM_GAS_LIMIT_MINIMUM                5000
  ETHEREUM_GAS_LIMIT_MAXIMUM                0xffffffffffffffff ;; maxUint64
  LINEA_GAS_LIMIT_MINIMUM                   61000000
  LINEA_GAS_LIMIT_MAXIMUM                   2000000000
  LINEA_BLOCK_GAS_LIMIT                     LINEA_GAS_LIMIT_MAXIMUM
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;               ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; SIZE / LENGTH ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;               ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  MMEDIUM                                   8
  MMEDIUMMO                                 (- MMEDIUM 1)
  LLARGE                                    16
  LLARGEMO                                  (- LLARGE 1)
  LLARGEPO                                  (+ LLARGE 1)
  WORD_SIZE                                 32
  WORD_SIZE_MO                              (- WORD_SIZE 1)
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;         ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; SHIFT   ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;         ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  TWOFIFTYSIX_TO_THE_FOUR                   (^ 256 4)
  TWOFIFTYSIX_TO_THE_FOUR_MO                (- TWOFIFTYSIX_TO_THE_FOUR 1)
  TWOFIFTYSIX_TO_THE_FIVE                   (^ 256 5)
  TWOFIFTYSIX_TO_THE_TWELVE                 (^ 256 12)
  TWOFIFTYSIX_TO_THE_TWELVE_MO              (- TWOFIFTYSIX_TO_THE_TWELVE 1)
  TWOFIFTYSIX_TO_THE_TWENTY                 (^ 256 20)
  TWOFIFTYSIX_TO_THE_TWENTY_MO              (- TWOFIFTYSIX_TO_THE_TWENTY 1)
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;               ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; PRECOMPILES   ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;               ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  MAX_PRC_ADDRESS                           9                        ;;London value
  PRC_ECPAIRING_SIZE                        (* 6 WORD_SIZE)
  PRC_BLAKE2F_SIZE                          213
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;         ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; EXO SUM ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;         ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  EXO_SUM_INDEX_ROM                         0
  EXO_SUM_INDEX_KEC                         1
  EXO_SUM_INDEX_LOG                         2
  EXO_SUM_INDEX_TXCD                        3                                  ;; for the transaction call data as found in RLP_TXN
  EXO_SUM_INDEX_ECDATA                      4
  EXO_SUM_INDEX_RIPSHA                      5
  EXO_SUM_INDEX_BLAKEMODEXP                 6
  EXO_SUM_WEIGHT_ROM                        (^ 2 EXO_SUM_INDEX_ROM)
  EXO_SUM_WEIGHT_KEC                        (^ 2 EXO_SUM_INDEX_KEC)
  EXO_SUM_WEIGHT_LOG                        (^ 2 EXO_SUM_INDEX_LOG)
  EXO_SUM_WEIGHT_TXCD                       (^ 2 EXO_SUM_INDEX_TXCD)
  EXO_SUM_WEIGHT_ECDATA                     (^ 2 EXO_SUM_INDEX_ECDATA)
  EXO_SUM_WEIGHT_RIPSHA                     (^ 2 EXO_SUM_INDEX_RIPSHA)
  EXO_SUM_WEIGHT_BLAKEMODEXP                (^ 2 EXO_SUM_INDEX_BLAKEMODEXP)
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; HASH CONSTANTS ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  EMPTY_KECCAK_HI                           0xc5d2460186f7233c927e7db2dcc703c0 ;; high part of KECCAK(())
  EMPTY_KECCAK_LO                           0xe500b653ca82273b7bfad8045d85a470 ;; low  part of KECCAK(())
  EMPTY_SHA2_HI                             0xe3b0c44298fc1c149afbf4c8996fb924 ;; high part of SHA2-256(())
  EMPTY_SHA2_LO                             0x27ae41e4649b934ca495991b7852b855 ;; low  part of SHA2-256(())
  EMPTY_RIPEMD_HI                           0x0000000000000000000000009c1185a5 ;; high part of RIPEMD-160(())
  EMPTY_RIPEMD_LO                           0xc5e9fc54612808977ee8f548b2258d31 ;; low  part of RIPEMD-160(())
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                     ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; BLAKE MODEXP MODULE ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                     ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  PHASE_MODEXP_BASE                         1
  PHASE_MODEXP_EXPONENT                     2
  PHASE_MODEXP_MODULUS                      3
  PHASE_MODEXP_RESULT                       4
  PHASE_BLAKE_DATA                          5
  PHASE_BLAKE_PARAMS                        6
  PHASE_BLAKE_RESULT                        7
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; EC DATA MODULE ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  PHASE_ECRECOVER_DATA                      0x010A
  PHASE_ECRECOVER_RESULT                    0x010B
  PHASE_ECADD_DATA                          0x060A
  PHASE_ECADD_RESULT                        0x060B
  PHASE_ECMUL_DATA                          0x070A
  PHASE_ECMUL_RESULT                        0x070B
  PHASE_ECPAIRING_DATA                      0x080A
  PHASE_ECPAIRING_RESULT                    0x080B
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;            ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; EXP MODULE ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;            ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  EXP_INST_EXPLOG                           0xEE0A
  EXP_INST_MODEXPLOG                        0xEE05
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;            ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; MMU MODULE ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;            ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;
  ;;MMU Instructions
  ;;
  MMU_INST_MLOAD                            0xfe01
  MMU_INST_MSTORE                           0xfe02
  MMU_INST_MSTORE8                          0xfe03
  MMU_INST_INVALID_CODE_PREFIX              0xfe00
  MMU_INST_RIGHT_PADDED_WORD_EXTRACTION     0xfe10
  MMU_INST_RAM_TO_EXO_WITH_PADDING          0xfe20
  MMU_INST_EXO_TO_RAM_TRANSPLANTS           0xfe30
  MMU_INST_RAM_TO_RAM_SANS_PADDING          0xfe40
  MMU_INST_ANY_TO_RAM_WITH_PADDING          0xfe50
  ;;MMU_INST_ANY_TO_RAM_WITH_PADDING_SOME_DATA    0xfe51
  ;;MMU_INST_ANY_TO_RAM_WITH_PADDING_PURE_PADDING 0xfe52
  MMU_INST_MODEXP_ZERO                      0xfe60
  MMU_INST_MODEXP_DATA                      0xfe70
  MMU_INST_BLAKE                            0xfe80
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;             ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; MMIO MODULE ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;             ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;
  ;; MMIO Instructions
  ;;
  MMIO_INST_LIMB_VANISHES                   0xfe01
  MMIO_INST_LIMB_TO_RAM_TRANSPLANT          0xfe11
  MMIO_INST_LIMB_TO_RAM_ONE_TARGET          0xfe12
  MMIO_INST_LIMB_TO_RAM_TWO_TARGET          0xfe13
  MMIO_INST_RAM_TO_LIMB_TRANSPLANT          0xfe21
  MMIO_INST_RAM_TO_LIMB_ONE_SOURCE          0xfe22
  MMIO_INST_RAM_TO_LIMB_TWO_SOURCE          0xfe23
  MMIO_INST_RAM_TO_RAM_TRANSPLANT           0xfe31
  MMIO_INST_RAM_TO_RAM_PARTIAL              0xfe32
  MMIO_INST_RAM_TO_RAM_TWO_TARGET           0xfe33
  MMIO_INST_RAM_TO_RAM_TWO_SOURCE           0xfe34
  MMIO_INST_RAM_EXCISION                    0xfe41
  MMIO_INST_RAM_VANISHES                    0xfe42
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;            ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; MXP MODULE ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;            ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  MISC_WEIGHT_EXP                           1
  MISC_WEIGHT_MMU                           2
  MISC_WEIGHT_MXP                           4
  MISC_WEIGHT_OOB                           8
  MISC_WEIGHT_STP                           16
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;            ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; OOB MODULE ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;            ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  OOB_INST_JUMP                             0x56
  OOB_INST_JUMPI                            0x57
  OOB_INST_RDC                              0x3E
  OOB_INST_CDL                              0x35
  OOB_INST_XCALL                            0xCC
  OOB_INST_CALL                             0xCA
  OOB_INST_CREATE                           0xCE
  OOB_INST_SSTORE                           0x55
  OOB_INST_DEPLOYMENT                       0xF3
  OOB_INST_ECRECOVER                        0xFF01
  OOB_INST_SHA2                             0xFF02
  OOB_INST_RIPEMD                           0xFF03
  OOB_INST_IDENTITY                         0xFF04
  OOB_INST_ECADD                            0xFF06
  OOB_INST_ECMUL                            0xFF07
  OOB_INST_ECPAIRING                        0xFF08
  OOB_INST_BLAKE_CDS                        0xFA09
  OOB_INST_BLAKE_PARAMS                     0xFB09
  OOB_INST_MODEXP_CDS                       0xFA05
  OOB_INST_MODEXP_XBS                       0xFB05
  OOB_INST_MODEXP_LEAD                      0xFC05
  OOB_INST_MODEXP_PRICING                   0xFD05
  OOB_INST_MODEXP_EXTRACT                   0xFE05
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;             ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; RLP* MODULE ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;             ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;
  ;; RLP prefix
  ;;
  RLP_PREFIX_INT_SHORT                      128                                ;;RLP prefix of a short integer (<56 bytes), defined in the EYP.
  RLP_PREFIX_INT_LONG                       183                                ;;RLP prefix of a long integer (>55 bytes), defined in the EYP.
  RLP_PREFIX_LIST_SHORT                     192                                ;;RLP prefix of a short list (<56 bytes), defined in the EYP.
  RLP_PREFIX_LIST_LONG                      247                                ;;RLP prefix of a long list (>55 bytes), defined in the EYP.
  ;;
  ;; RLP_TXN Phase
  ;;
  RLP_TXN_PHASE_RLP_PREFIX                  1
  RLP_TXN_PHASE_CHAIN_ID                    2
  RLP_TXN_PHASE_NONCE                       3
  RLP_TXN_PHASE_GAS_PRICE                   4
  RLP_TXN_PHASE_MAX_PRIORITY_FEE_PER_GAS    5
  RLP_TXN_PHASE_MAX_FEE_PER_GAS             6
  RLP_TXN_PHASE_GAS_LIMIT                   7
  RLP_TXN_PHASE_TO                          8
  RLP_TXN_PHASE_VALUE                       9
  RLP_TXN_PHASE_DATA                        10
  RLP_TXN_PHASE_ACCESS_LIST                 11
  RLP_TXN_PHASE_BETA                        12
  RLP_TXN_PHASE_Y                           13
  RLP_TXN_PHASE_R                           14
  RLP_TXN_PHASE_S                           15
  ;;
  ;; RLP_RCPT Phase
  ;;
  RLP_RCPT_SUBPHASE_ID_TYPE                 7
  RLP_RCPT_SUBPHASE_ID_STATUS_CODE          2
  RLP_RCPT_SUBPHASE_ID_CUMUL_GAS            3
  RLP_RCPT_SUBPHASE_ID_NO_LOG_ENTRY         11
  RLP_RCPT_SUBPHASE_ID_ADDR                 53
  RLP_RCPT_SUBPHASE_ID_TOPIC_BASE           65
  RLP_RCPT_SUBPHASE_ID_DATA_LIMB            77
  RLP_RCPT_SUBPHASE_ID_DATA_SIZE            83
  RLP_RCPT_SUBPHASE_ID_TOPIC_DELTA          96
  ;;
  ;; RLP_ADDR
  ;;
  RLP_ADDR_RECIPE_1                         1                                  ;; for RlpAddr, used to discriminate between recipe for create
  RLP_ADDR_RECIPE_2                         2                                  ;; for RlpAddr, used to discriminate between recipe for create
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; SHAKIRA MODULE ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;
  ;; SHAKIRA PHASE
  ;;
  PHASE_SHA2_DATA                           1
  PHASE_SHA2_RESULT                         2
  PHASE_RIPEMD_DATA                         3
  PHASE_RIPEMD_RESULT                       4
  PHASE_KECCAK_DATA                         5
  PHASE_KECCAK_RESULT                       6
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;            ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; WCP MODULE ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;            ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  WCP_INST_GEQ                              0x0E
  WCP_INST_LEQ                              0x0F)


