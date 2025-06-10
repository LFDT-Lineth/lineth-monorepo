(deflookup
  txndata-into-rlptxn
  ;; target columns
  (
    rlptxn.ABS_TX_NUM_INFINY
    rlptxn.ABS_TX_NUM
    rlptxn.CODE_FRAGMENT_INDEX
    rlptxn.PHASE
    rlptxn.DATA_HI
    rlptxn.DATA_LO
  )
  ;; source columns
  (
    txndata.ABS_TX_NUM_MAX
    txndata.ABS_TX_NUM
    txndata.CODE_FRAGMENT_INDEX
    txndata.PHASE_RLP_TXN
    txndata.OUTGOING_HI
    txndata.OUTGOING_LO
  ))


