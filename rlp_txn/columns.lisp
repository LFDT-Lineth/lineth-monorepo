(module rlpTxn)

(defcolumns 
  ABS_TX_NUM
  LIMB
  nBYTES
  (LIMB_CONSTRUCTED :binary)
  (LT :binary)
  (LX :binary)
  INDEX_LT
  INDEX_LX
  ABS_TX_NUM_INFINY
  DATA_HI
  DATA_LO
  CODE_FRAGMENT_INDEX
  (REQUIRES_EVM_EXECUTION :binary)
  (PHASE :binary :array [0:14])
  (PHASE_END :binary)
  TYPE
  COUNTER
  (DONE :binary)
  nSTEP
  (INPUT :display :bytes :array [2])
  (BYTE :byte :array [2])
  (ACC :display :bytes :array [2])
  ACC_BYTESIZE
  (BIT :binary)
  BIT_ACC
  POWER
  RLP_LT_BYTESIZE
  RLP_LX_BYTESIZE
  (LC_CORRECTION :binary)
  (IS_PREFIX :binary)
  PHASE_SIZE
  INDEX_DATA
  DATAGASCOST
  (DEPTH :binary :array [2])
  ADDR_HI
  ADDR_LO
  ACCESS_TUPLE_BYTESIZE
  nADDR
  nKEYS
  nKEYS_PER_ADDR)

;; aliases
(defalias 
  CT  COUNTER
  LC  LIMB_CONSTRUCTED
  P   POWER
  CFI CODE_FRAGMENT_INDEX)


