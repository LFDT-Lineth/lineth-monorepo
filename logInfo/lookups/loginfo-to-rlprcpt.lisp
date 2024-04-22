(deflookup 
  logInfo-into-rlpTxnRcpt
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
    logInfo.ABS_TXN_NUM_MAX
    logInfo.ABS_TXN_NUM
    logInfo.ABS_LOG_NUM_MAX
    logInfo.ABS_LOG_NUM
    logInfo.PHASE
    logInfo.DATA_HI
    logInfo.DATA_LO
  ))


