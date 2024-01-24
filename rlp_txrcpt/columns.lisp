(module rlpTxRcpt)

(defcolumns 
  (ABS_TX_NUM :i16)
  (ABS_TX_NUM_MAX :i16)
  (ABS_LOG_NUM :i24)
  (ABS_LOG_NUM_MAX :i24)
  (LIMB :i128 :display :bytes)
  (nBYTES :byte)
  (LIMB_CONSTRUCTED :binary@prove)
  (INDEX :i16)
  (INDEX_LOCAL :i16)
  (PHASE :binary@prove :array [5])
  (PHASE_END :binary@prove)
  (COUNTER :byte)
  (nSTEP :byte)
  (DONE :binary@prove)
  (TXRCPT_SIZE :i32)
  (INPUT :i16 :display :bytes :array [4])
  (BYTE :byte@prove :array [4])
  (ACC :i128 :display :bytes :array [4])
  (ACC_SIZE :byte)
  (BIT :binary@prove)
  (BIT_ACC :byte)
  (POWER :i128)
  (IS_PREFIX :binary@prove)
  (LC_CORRECTION :binary@prove)
  (PHASE_SIZE :i32)
  (DEPTH_1 :binary@prove)
  (IS_TOPIC :binary@prove)
  (IS_DATA :binary@prove)
  (LOG_ENTRY_SIZE :i32)
  (LOCAL_SIZE :i32))

;; aliases
(defalias 
  CT COUNTER
  LC LIMB_CONSTRUCTED
  P  POWER)


