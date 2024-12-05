(deflookup
  stp-into-wcp
  ; target colums (in WCP)
  (
    wcp.ARG_1_HI
    wcp.ARG_1_LO
    wcp.ARG_2_HI
    wcp.ARG_2_LO
    wcp.RES
    wcp.INST
  )
  ; source columns (in STP)
  (
    (* stp.ARG_1_HI stp.WCP_FLAG)
    (* stp.ARG_1_LO stp.WCP_FLAG)
    0
    (* stp.ARG_2_LO stp.WCP_FLAG)
    (* stp.RES_LO stp.WCP_FLAG)
    (* stp.EXO_INST stp.WCP_FLAG)
  ))


