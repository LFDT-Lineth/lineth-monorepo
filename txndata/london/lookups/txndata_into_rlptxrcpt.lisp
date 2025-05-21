(deflookup
  txndata-into-rlptxrcpt
  ;; target columns
  (
    rlptxrcpt.ABS_TX_NUM_MAX
    rlptxrcpt.ABS_TX_NUM
    rlptxrcpt.PHASE_ID
    [rlptxrcpt.INPUT 1]
  )
  ;; source columns
  (
    (* txndata.ABS_TX_NUM_MAX (~ txndata.PHASE_RLP_TXNRCPT))
    (* txndata.ABS_TX_NUM (~ txndata.PHASE_RLP_TXNRCPT))
    (* txndata.PHASE_RLP_TXNRCPT (~ txndata.PHASE_RLP_TXNRCPT))
    (* txndata.OUTGOING_RLP_TXNRCPT (~ txndata.PHASE_RLP_TXNRCPT))
  ))


