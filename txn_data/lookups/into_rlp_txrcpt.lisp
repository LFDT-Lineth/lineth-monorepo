(deflookup 
  txnData-into-rlpTxnRcpt
  ;; target columns
  (
    rlpTxRcpt.ABS_TX_NUM_MAX
    rlpTxRcpt.ABS_TX_NUM
    rlpTxRcpt.PHASE_ID
    [rlpTxRcpt.INPUT 1]
  )
  ;; source columns
  (
    (* txnData.ABS_TX_NUM_MAX (~ txnData.PHASE_RLP_TXNRCPT))
    (* txnData.ABS_TX_NUM (~ txnData.PHASE_RLP_TXNRCPT))
    (* txnData.PHASE_RLP_TXNRCPT (~ txnData.PHASE_RLP_TXNRCPT))
    (* txnData.OUTGOING_RLP_TXNRCPT (~ txnData.PHASE_RLP_TXNRCPT))
  ))


