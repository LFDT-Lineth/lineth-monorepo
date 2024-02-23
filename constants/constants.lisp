(defconst 
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                 ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; EVM INSTRUCTION ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                 ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ADD                                           1
  MUL                                           2
  SUB                                           3
  DIV                                           4
  SDIV                                          5
  MOD                                           6
  SMOD                                          7
  ADDMOD                                        8
  MULMOD                                        9
  EXP                                           10
  SIGNEXTEND                                    11
  LT                                            16
  GT                                            17
  SLT                                           18
  SGT                                           19
  EQ_                                           20
  ISZERO                                        21
  AND                                           22
  OR                                            23
  XOR                                           24
  NOT                                           25
  BYTE                                          26
  SHL                                           27
  SHR                                           28
  SAR                                           29
  LOG0                                          0xa0
  LOG1                                          0xa1
  LOG2                                          0xa2
  LOG3                                          0xa3
  LOG4                                          0xa4
  INVALID_CODE_PREFIX_VALUE                     0xEF
  JUMPDEST                                      0x5b
  PUSH_1                                        0x60
  INVALID_OPCODE                                0xfe
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;               ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; SIZE / LENGTH ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;               ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  MMEDIUMMO                                     7
  MMEDIUM                                       8
  LLARGEMO                                      15
  LLARGE                                        16
  LLARGEPO                                      (+ LLARGE 1)
  WORD_SIZE_MO                                  31
  WORD_SIZE                                     32
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                     ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; BLAKE MODEXP MODULE ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                     ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  MODEXP_PHASE_BASE                             1
  MODEXP_PHASE_EXPONENT                         2
  MODEXP_PHASE_MODULUS                          3
  MODEXP_PHASE_RESULT                           4
  BLAKE_PHASE_DATA                              5
  BLAKE_PHASE_PARAMS                            6
  BLAKE_PHASE_RESULT                            7
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;            ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; MMU MODULE ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;            ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;
  ;;MMU Instructions
  ;;
  MMU_INST_MLOAD                                0xfe01
  MMU_INST_MSTORE                               0xfe02
  MMU_INST_MSTORE8                              0x53
  MMU_INST_INVALID_CODE_PREFIX                  0xfe00
  MMU_INST_RIGHT_PADDED_WORD_EXTRACTION         0xfe10
  MMU_INST_RAM_TO_EXO_WITH_PADDING              0xfe20
  MMU_INST_EXO_TO_RAM_TRANSPLANTS               0xfe30
  MMU_INST_RAM_TO_RAM_SANS_PADDING              0xfe40
  MMU_INST_ANY_TO_RAM_WITH_PADDING              0xfe50
  MMU_INST_ANY_TO_RAM_WITH_PADDING_SOME_DATA    0xfe51
  MMU_INST_ANY_TO_RAM_WITH_PADDING_PURE_PADDING 0xfe52
  MMU_INST_MODEXP_ZERO                          0xfe60
  MMU_INST_MODEXP_DATA                          0xfe70
  MMU_INST_BLAKE                                0xfe80
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;             ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; MMIO MODULE ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;             ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;
  ;; MMIO Instructions
  ;;
  ;; LIMB
  MMIO_INST_LIMB_VANISHES                       0xfe01
  ;; LIMB to RAM
  MMIO_INST_LIMB_TO_RAM_TRANSPLANT              0xfe11
  MMIO_INST_LIMB_TO_RAM_ONE_TARGET              0xfe12
  MMIO_INST_LIMB_TO_RAM_TWO_TARGET              0xfe13
  ;; Ram to LIMB
  MMIO_INST_RAM_TO_LIMB_TRANSPLANT              0xfe21
  MMIO_INST_RAM_TO_LIMB_ONE_SOURCE              0xfe22
  MMIO_INST_RAM_TO_LIMB_TWO_SOURCE              0xfe23
  ;; RAM to RAM
  MMIO_INST_RAM_TO_RAM_TRANSPLANT               0xfe31
  MMIO_INST_RAM_TO_RAM_PARTIAL                  0xfe32
  MMIO_INST_RAM_TO_RAM_TWO_TARGET               0xfe33
  MMIO_INST_RAM_TO_RAM_TWO_SOURCE               0xfe34
  ;; RAM
  MMIO_INST_RAM_EXCISION                        0xfe41
  MMIO_INST_RAM_VANISHES                        0xfe42
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;             ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; RLP* MODULE ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;             ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;
  ;; RLP prefix
  ;;
  RLP_PREFIX_INT_SHORT                          128 ;;RLP prefix of a short integer (<56 bytes), defined in the EYP.
  RLP_PREFIX_INT_LONG                           183 ;;RLP prefix of a long integer (>55 bytes), defined in the EYP.
  RLP_PREFIX_LIST_SHORT                         192 ;;RLP prefix of a short list (<56 bytes), defined in the EYP.
  RLP_PREFIX_LIST_LONG                          247 ;;RLP prefix of a long list (>55 bytes), defined in the EYP.
  ;;
  ;; RLP_TXN Phase
  ;;
  RLP_TXN_PHASE_RLP_PREFIX_VALUE                1
  RLP_TXN_PHASE_CHAIN_ID_VALUE                  2
  RLP_TXN_PHASE_NONCE_VALUE                     3
  RLP_TXN_PHASE_GAS_PRICE_VALUE                 4
  RLP_TXN_PHASE_MAX_PRIORITY_FEE_PER_GAS_VALUE  5
  RLP_TXN_PHASE_MAX_FEE_PER_GAS_VALUE           6
  RLP_TXN_PHASE_GAS_LIMIT_VALUE                 7
  RLP_TXN_PHASE_TO_VALUE                        8
  RLP_TXN_PHASE_VALUE_VALUE                     9
  RLP_TXN_PHASE_DATA_VALUE                      10
  RLP_TXN_PHASE_ACCESS_LIST_VALUE               11
  RLP_TXN_PHASE_BETA_VALUE                      12
  RLP_TXN_PHASE_Y_VALUE                         13
  RLP_TXN_PHASE_R_VALUE                         14
  RLP_TXN_PHASE_S_VALUE                         15
  ;;
  ;; RLP_RCPT Phase
  ;;
  RLP_RCPT_SUBPHASE_ID_TYPE                     7
  RLP_RCPT_SUBPHASE_ID_STATUS_CODE              2
  RLP_RCPT_SUBPHASE_ID_CUMUL_GAS                3
  RLP_RCPT_SUBPHASE_ID_NO_LOG_ENTRY             11
  RLP_RCPT_SUBPHASE_ID_ADDR                     53
  RLP_RCPT_SUBPHASE_ID_TOPIC_BASE               65
  RLP_RCPT_SUBPHASE_ID_DATA_LIMB                77
  RLP_RCPT_SUBPHASE_ID_DATA_SIZE                83
  RLP_RCPT_SUBPHASE_ID_TOPIC_DELTA              96
  ;;
  ;; RLP_ADDR 
  ;;
  RLP_ADDR_RECIPE_1                             1   ;; for RlpAddr, used to discriminate between recipe for create
  RLP_ADDR_RECIPE_2                             2   ;; for RlpAddr, used to discriminate between recipe for create
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;            ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;; WCP MODULE ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;            ;;
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  LEQ                                           0x0E
  GEQ                                           0x0F)


