(module rlptxrcpt)

(defcolumns
  (ABS_TX_NUM       :i32)
  (ABS_TX_NUM_MAX   :i32)
  (ABS_LOG_NUM      :i32)
  (ABS_LOG_NUM_MAX  :i32)
  (LIMB             :i128 :display :bytes)
  (nBYTES           :i5)
  (LIMB_CONSTRUCTED :binary@prove)
  (INDEX            :i24)
  (INDEX_LOCAL      :i24)
  (PHASE            :binary@prove :array [5])
  (PHASE_END        :binary@prove)
  (COUNTER          :i32)
  (nSTEP            :i32)
  (DONE             :binary)
  (TXRCPT_SIZE      :i32)
  (INPUT            :i128 :display :bytes :array [4])
  (BYTE             :byte@prove :array [4])
  (ACC              :i128 :display :bytes :array [4])
  (ACC_SIZE         :i5)
  (BIT              :binary@prove)
  (BIT_ACC          :byte)
  (POWER            :i128)
  (IS_PREFIX        :binary@prove)
  (LC_CORRECTION    :binary@prove)
  (PHASE_SIZE       :i32)
  (DEPTH_1          :binary@prove)
  (IS_TOPIC         :binary@prove)
  (IS_DATA          :binary@prove)
  (LOG_ENTRY_SIZE   :i32)
  (LOCAL_SIZE       :i32)
  (PHASE_ID         :i16))

;; aliases
(defalias
  CT COUNTER
  LC LIMB_CONSTRUCTED
  P  POWER)


