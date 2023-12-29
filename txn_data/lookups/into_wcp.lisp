(deflookup 
  txn_data_into_wcp
  ; target columns
  (
    wcp.ARGUMENT_1_HI
    wcp.ARGUMENT_1_LO
    wcp.ARGUMENT_2_HI
    wcp.ARGUMENT_2_LO
    wcp.RESULT
    wcp.INST
  )
  ; source columns
  (
    0
    txnData.WCP_ARG_ONE_LO
    0
    txnData.WCP_ARG_TWO_LO
    txnData.WCP_RES
    txnData.WCP_INST
  ))


