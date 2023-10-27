(defplookup 
  txnData-into-rlpTxn
  ;reference columns
  (
    rlpTxn.ABS_TX_NUM_INFINY
    rlpTxn.ABS_TX_NUM
    (reduce +
            (for i [0 : 14] (* i [rlpTxn.PHASE i])))
    rlpTxn.DATA_HI
    rlpTxn.DATA_LO
  )
  ;source columns
  (
    txnData.ABS_TX_NUM_MAX
    txnData.ABS_TX_NUM
    txnData.PHASE_RLP_TXN
    txnData.OUTGOING_HI
    txnData.OUTGOING_LO
  ))


