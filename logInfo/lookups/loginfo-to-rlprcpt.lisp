(deflookup 
  logInfo-into-rlpTxnRcpt
  ;reference columns
  (
    rlpTxRcpt.ABS_TX_NUM_MAX
    rlpTxRcpt.ABS_TX_NUM
    rlpTxRcpt.ABS_LOG_NUM_MAX
    rlpTxRcpt.ABS_LOG_NUM
    (subphaseId-rlp-txrcpt)
    [rlpTxRcpt.INPUT 1]
    [rlpTxRcpt.INPUT 2]
  )
  ;source columns
  (
    logInfo.ABS_TXN_NUM_MAX
    logInfo.ABS_TXN_NUM
    logInfo.ABS_LOG_NUM_MAX
    logInfo.ABS_LOG_NUM
    logInfo.PHASE
    logInfo.DATA_HI
    logInfo.DATA_LO
  ))


