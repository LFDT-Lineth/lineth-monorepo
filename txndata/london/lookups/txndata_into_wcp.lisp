(deflookup
  txndata-into-wcp
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
    (* txndata.WCP_FLAG txndata.ARG_ONE_LO)
    0
    (* txndata.WCP_FLAG txndata.ARG_TWO_LO)
    (* txndata.WCP_FLAG txndata.RES)
    (* txndata.WCP_FLAG txndata.INST)
  ))


