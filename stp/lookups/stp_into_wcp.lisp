(defclookup
  (stp-into-wcp :unchecked)
  ; target colums (in WCP)
  (
    wcp.ARG_1_HI
    wcp.ARG_1_LO
    wcp.ARG_2_HI
    wcp.ARG_2_LO
    wcp.RES
    wcp.INST
  )
  ; source selector
  stp.WCP_FLAG
  ; source columns (in STP)
  (
    stp.ARG_1_HI
    stp.ARG_1_LO
    0
    stp.ARG_2_LO
    stp.RES_LO
    stp.EXO_INST
  ))


