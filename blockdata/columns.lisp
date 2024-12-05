(module blockdata)

(defcolumns
  (FIRST_BLOCK_NUMBER :i48)
  (CT :i4)
  (REL_BLOCK           :i10)
  (REL_TX_NUM_MAX      :i10)
  (INST :byte :display :opcode)
  (DATA_HI :i128)
  (DATA_LO :i128)
  (COINBASE_HI :i32)
  (COINBASE_LO :i128)
  (BLOCK_GAS_LIMIT :i48)
  (BASEFEE :i48)
  (BYTE_HI :array [0:15] :byte@prove)
  (BYTE_LO :array [0:15] :byte@prove)
  (WCP_FLAG :binary))

