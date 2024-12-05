(deflookup
  loginfo-into-rlptxrcpt
  ;; target columns
  (
    rlptxrcpt.ABS_TX_NUM_MAX
    rlptxrcpt.ABS_TX_NUM
    rlptxrcpt.ABS_LOG_NUM_MAX
    rlptxrcpt.ABS_LOG_NUM
    rlptxrcpt.PHASE_ID
    [rlptxrcpt.INPUT 1]
    [rlptxrcpt.INPUT 2]
  )
  ;; source columns
  (
    loginfo.ABS_TXN_NUM_MAX
    loginfo.ABS_TXN_NUM
    loginfo.ABS_LOG_NUM_MAX
    loginfo.ABS_LOG_NUM
    loginfo.PHASE
    loginfo.DATA_HI
    loginfo.DATA_LO
  ))


