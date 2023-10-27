(defpurefun (subphaseId-rlp-txrcpt)
  (+ (reduce +
             (for i [1 : 5] (* i [rlpTxRcpt.PHASE i])))
     (* 6 rlpTxRcpt.IS_PREFIX)
     (* 12 rlpTxRcpt.IS_TOPIC)
     (* 24 rlpTxRcpt.IS_DATA)
     (* 48 rlpTxRcpt.DEPTH_1)
     (* 96 rlpTxRcpt.IS_TOPIC rlpTxRcpt.INDEX_LOCAL)))

(defplookup 
  txnData-into-rlpTxnRcpt
  ;reference columns
  (
    rlpTxRcpt.ABS_TX_NUM_MAX
    rlpTxRcpt.ABS_TX_NUM
    (subphaseId-rlp-txrcpt)
    [rlpTxRcpt.INPUT 1]
  )
  ;source columns
  (
    (* txnData.ABS_TX_NUM_MAX (~ txnData.PHASE_RLP_TXNRCPT))
    (* txnData.ABS_TX_NUM (~ txnData.PHASE_RLP_TXNRCPT))
    (* txnData.PHASE_RLP_TXNRCPT (~ txnData.PHASE_RLP_TXNRCPT))
    (* txnData.OUTGOING_RLP_TXNRCPT (~ txnData.PHASE_RLP_TXNRCPT))
  ))


