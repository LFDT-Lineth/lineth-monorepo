(module ec_data)

(defcolumns 
  STAMP
  INDEX
  CT_MIN
  LIMB
  TYPE
  (EC_RECOVER :binary)
  (EC_ADD :binary)
  (EC_MUL :binary)
  (EC_PAIRING :binary)
  TOTAL_PAIRINGS
  ACC_PAIRINGS
  (COMPARISONS :binary)
  (EQUALITIES :binary)
  (HURDLE :binary)
  (PRELIMINARY_CHECKS_PASSED :binary)
  (ALL_CHECKS_PASSED :binary)
  SQUARE
  CUBE
  (BYTE_DELTA :byte)
  ACC_DELTA
  WCP_ARG1_HI
  WCP_ARG1_LO
  WCP_ARG2_HI
  WCP_ARG2_LO
  (WCP_INST :byte :display :opcode)
  (WCP_RES :binary)
  EXT_ARG1_HI
  EXT_ARG1_LO
  EXT_ARG2_HI
  EXT_ARG2_LO
  EXT_ARG3_HI
  EXT_ARG3_LO
  EXT_INST
  EXT_RES_LO
  EXT_RES_HI
  (THIS_IS_NOT_ON_G2 :binary)
  (THIS_IS_NOT_ON_G2_ACC :binary)
  (SOMETHING_WASNT_ON_G2 :binary))

;; aliases
(defalias 
  PCP PRELIMINARY_CHECKS_PASSED)


