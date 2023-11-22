(defpurefun (subphaseId-rlp-txrcpt)
  (+ (reduce +
             (for i [1 : 5] (* i [rlpTxRcpt.PHASE i])))
     (* 6 rlpTxRcpt.IS_PREFIX)
     (* 12 rlpTxRcpt.IS_TOPIC)
     (* 24 rlpTxRcpt.IS_DATA)
     (* 48 rlpTxRcpt.DEPTH_1)
     (* 96 rlpTxRcpt.IS_TOPIC rlpTxRcpt.INDEX_LOCAL))) ;;TODO, doublon, already define in txnData->rlpRcpt lookup

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


