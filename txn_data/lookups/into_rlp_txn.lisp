(deflookup 
  txnData-into-rlpTxn
  ;; target columns
  (
    rlpTxn.ABS_TX_NUM_INFINY
    rlpTxn.ABS_TX_NUM
    rlpTxn.PHASE_ID
    rlpTxn.DATA_HI
    rlpTxn.DATA_LO
  )
  ;; source columns
  (
    txnData.ABS_TX_NUM_MAX
    txnData.ABS_TX_NUM
    txnData.PHASE_RLP_TXN
    txnData.OUTGOING_HI
    txnData.OUTGOING_LO
  ))


