(defclookup
  (txndata-into-wcp :unchecked)
  ; target columns
  (
    wcp.ARG_1
    wcp.ARG_2
    wcp.RES
    wcp.INST
    )
  ; source selector
  txndata.WCP_FLAG
  ; source columns
  (
    txndata.ARG_ONE_LO
    txndata.ARG_TWO_LO
    txndata.RES
    txndata.INST
  ))


