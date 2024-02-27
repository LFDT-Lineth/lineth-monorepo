(module hub_v2)

;; PHASE numbers for lookups
(defconst 
  PHASE_TRANSACTION_CALL_DATA 9
  PHASE_ECRECOVER_DATA        iota
  PHASE_ECRECOVER_RESULT      iota
  PHASE_SHA2-256_DATA         iota
  PHASE_SHA2-256_RESULT       iota
  PHASE_RIPEMD-160_DATA       iota
  PHASE_RIPEMD-160_RESULT     iota
  PHASE_MODEXP_BASE           iota ;; @Tsvetan: the BLKMXP module must sync with these values) 
  PHASE_MODEXP_EXPONENT       iota
  PHASE_MODEXP_MODULUS        iota
  PHASE_MODEXP_RESULT         iota
  PHASE_ECADD_DATA            iota
  PHASE_ECADD_RESULT          iota
  PHASE_ECMUL_DATA            iota
  PHASE_ECMUL_RESULT          iota
  PHASE_PAIRING_DATA          iota
  PHASE_PAIRING_RESULT        iota
  PHASE_BLAKE_PARAMETERS      iota ;; @Tsvetan: same
  PHASE_BLAKE_DATA            iota
  PHASE_BLAKE_RESULT          iota)

(defconst
  RETURNDATACOPY              0x3e
  SSTORE                      0x55
  SELFDESTRUCT                0xff
  RETURN                      0xf3)
