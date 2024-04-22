(deflookup 
  txnData-into-rlpTxn
  ;; target columns
  (
    rlptxn.ABS_TX_NUM_INFINY
    rlptxn.ABS_TX_NUM
    rlptxn.PHASE_ID
    rlptxn.DATA_HI
    rlptxn.DATA_LO
  )
  ;; source columns
  (
    txnData.ABS_TX_NUM_MAX
    txnData.ABS_TX_NUM
    txnData.PHASE_RLP_TXN
    txnData.OUTGOING_HI
    txnData.OUTGOING_LO
  ))


