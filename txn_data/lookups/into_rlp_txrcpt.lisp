(deflookup 
  txnData-into-rlpTxnRcpt
  ;; target columns
  (
    rlptxrcpt.ABS_TX_NUM_MAX
    rlptxrcpt.ABS_TX_NUM
    rlptxrcpt.PHASE_ID
    [rlptxrcpt.INPUT 1]
  )
  ;; source columns
  (
    (* txnData.ABS_TX_NUM_MAX (~ txnData.PHASE_RLP_TXNRCPT))
    (* txnData.ABS_TX_NUM (~ txnData.PHASE_RLP_TXNRCPT))
    (* txnData.PHASE_RLP_TXNRCPT (~ txnData.PHASE_RLP_TXNRCPT))
    (* txnData.OUTGOING_RLP_TXNRCPT (~ txnData.PHASE_RLP_TXNRCPT))
  ))


