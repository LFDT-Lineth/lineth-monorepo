(module loginfo)

(defcolumns
  (ABS_TXN_NUM_MAX :i24)
  (ABS_TXN_NUM :i24)
  (TXN_EMITS_LOGS :binary@prove)
  (ABS_LOG_NUM_MAX :i24)
  (ABS_LOG_NUM :i24)
  (CT_MAX :byte)
  (CT :byte)
  (ADDR_HI :i32)
  (ADDR_LO :i128)
  (TOPIC_HI :array [4])
  (TOPIC_LO :array [4])
  (DATA_SIZE :i32)
  (INST :byte :display :opcode)
  (IS_LOG_X :binary@prove :array [0:4])
  ;; lookup columns
  (PHASE :i16)
  (DATA_HI :i128)
  (DATA_LO :i128))


