(module rlpTxRcpt)

(defcolumns 
  ABS_TX_NUM
  ABS_TX_NUM_MAX
  ABS_LOG_NUM
  ABS_LOG_NUM_MAX
  (LIMB :display :bytes)
  (nBYTES :byte)
  (LIMB_CONSTRUCTED :binary)
  INDEX
  INDEX_LOCAL
  (PHASE :binary :array [5])
  (PHASE_END :binary)
  COUNTER
  nSTEP
  (DONE :binary)
  TXRCPT_SIZE
  (INPUT :display :bytes :array [4])
  (BYTE :byte :array [4])
  (ACC :display :bytes :array [4])
  ACC_SIZE
  (BIT :binary)
  (BIT_ACC :byte)
  POWER
  (IS_PREFIX :binary)
  (LC_CORRECTION :binary)
  PHASE_SIZE
  (DEPTH_1 :binary)
  (IS_TOPIC :binary)
  (IS_DATA :binary)
  LOG_ENTRY_SIZE
  LOCAL_SIZE)

;; aliases
(defalias 
  CT COUNTER
  LC LIMB_CONSTRUCTED
  P  POWER)


