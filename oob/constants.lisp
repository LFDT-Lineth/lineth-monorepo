(module oob)

(defconst 
  CT_MAX_JUMP             0
  CT_MAX_JUMPI            1
  CT_MAX_RDC              2
  CT_MAX_CDL              0
  CT_MAX_XCALL            0
  CT_MAX_CALL             2
  CT_MAX_CREATE           2
  CT_MAX_SSTORE           0
  CT_MAX_DEPLOYMENT       0
  CT_MAX_ECRECOVER        2
  CT_MAX_SHA2             3
  CT_MAX_RIPEMD           3
  CT_MAX_IDENTITY         3
  CT_MAX_ECADD            2
  CT_MAX_ECMUL            2
  CT_MAX_ECPAIRING        4
  CT_MAX_BLAKE2F_cds      1
  CT_MAX_BLAKE2F_params   1
  CT_MAX_MODEXP_cds       2
  CT_MAX_MODEXP_xbs       2
  CT_MAX_MODEXP_lead      3
  CT_MAX_MODEXP_pricing   5
  CT_MAX_MODEXP_extract   3
  LT                      0x10    ;; TODO: remove and replace by EVM_INST_XXX
  ISZERO                  0x15
  ADD                     0x01
  DIV                     0x04
  MOD                     0x06
  GT                      0x11
  EQ                      0x14
  G_CALLSTIPEND           2300   ;; TODO: remove and replace by GAS_CONST_G_XXX
  G_QUADDIVISOR           3)


