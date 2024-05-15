(module ecdata)

(defconst 
  P_BN_HI                       0x30644e72e131a029b85045b68181585d
  P_BN_LO                       0x97816a916871ca8d3c208c16d87cfd47
  SECP256K1N_HI                 0xffffffffffffffffffffffffffffffff
  SECP256K1N_LO                 0xfffffffffffffffffffffffefffffc2f
  LT                            0x10
  EQ                            0x14
  MULMOD                        0x09
  ADDMOD                        0x08
  ECRECOVER                     0x01
  ECADD                         0x06
  ECMUL                         0x07
  ECPAIRING                     0x08
  PHASE_ECRECOVER_DATA          0x010A
  PHASE_ECRECOVER_RESULT        0x010B
  PHASE_ECADD_DATA              0x060A
  PHASE_ECADD_RESULT            0x060B
  PHASE_ECMUL_DATA              0x070A
  PHASE_ECMUL_RESULT            0x070B
  PHASE_ECPAIRING_DATA          0x080A
  PHASE_ECPAIRING_RESULT        0x080B
  INDEX_MAX_ECRECOVER_DATA      7
  INDEX_MAX_ECADD_DATA          7
  INDEX_MAX_ECMUL_DATA          5
  INDEX_MAX_ECPAIRING_DATA_MIN  11
  INDEX_MAX_ECRECOVER_RESULT    1
  INDEX_MAX_ECADD_RESULT        3
  INDEX_MAX_ECMUL_RESULT        3
  INDEX_MAX_ECPAIRING_RESULT    1
  TOTAL_SIZE_ECRECOVER_DATA     128
  TOTAL_SIZE_ECADD_DATA         128
  TOTAL_SIZE_ECMUL_DATA         96
  TOTAL_SIZE_ECPAIRING_DATA_MIN 196
  TOTAL_SIZE_ECRECOVER_RESULT   32
  TOTAL_SIZE_ECADD_RESULT       64
  TOTAL_SIZE_ECMUL_RESULT       64
  TOTAL_SIZE_ECPAIRING_RESULT   32
  CT_MAX_SMALL_POINT            3
  CT_MAX_LARGE_POINT            7)


