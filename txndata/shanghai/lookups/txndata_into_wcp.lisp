(defclookup
  (txndata-into-wcp :unchecked)
  ;; target columns
  (
    wcp.ARGUMENT_1_HI
    wcp.ARGUMENT_1_LO
    wcp.ARGUMENT_2_HI
    wcp.ARGUMENT_2_LO
    wcp.RESULT
    wcp.INST
  )
  ;; source selector
  txndata.WCP_FLAG
  ;; source columns
  (
    0
    txndata.ARG_ONE_LO
    0
    txndata.ARG_TWO_LO
    txndata.RES
    txndata.INST
  ))


