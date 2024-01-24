(module rlpTxn)

(defcolumns 
  (ABS_TX_NUM :i24)
  (LIMB :i128)
  (nBYTES :byte)
  (LIMB_CONSTRUCTED :binary@prove)
  (LT :binary@prove)
  (LX :binary@prove)
  (INDEX_LT :i32)
  (INDEX_LX :i32)
  (ABS_TX_NUM_INFINY :i16)
  (DATA_HI :i128)
  (DATA_LO :i128)
  (CODE_FRAGMENT_INDEX :i24)
  (REQUIRES_EVM_EXECUTION :binary@prove)
  (PHASE :binary@prove :array [0:14])
  (PHASE_END :binary@prove)
  (TYPE :byte)
  (COUNTER :byte)
  (DONE :binary@prove)
  (nSTEP :byte)
  (INPUT :display :bytes :array [2])
  (BYTE :byte@prove :array [2])
  (ACC :display :bytes :array [2])
  (ACC_BYTESIZE :byte)
  (BIT :binary@prove)
  (BIT_ACC :byte)
  (POWER :i128)
  (RLP_LT_BYTESIZE :i24)
  (RLP_LX_BYTESIZE :i24)
  (LC_CORRECTION :binary@prove)
  (IS_PREFIX :binary@prove)
  (PHASE_SIZE :i24)
  (INDEX_DATA :i24)
  (DATAGASCOST :i32)
  (DEPTH :binary@prove :array [2])
  (ADDR_HI :i32)
  (ADDR_LO :i128)
  (ACCESS_TUPLE_BYTESIZE :i3)
  (nADDR :i16)
  (nKEYS :i16)
  (nKEYS_PER_ADDR :i16))

;; aliases
(defalias 
  CT  COUNTER
  LC  LIMB_CONSTRUCTED
  P   POWER
  CFI CODE_FRAGMENT_INDEX)


