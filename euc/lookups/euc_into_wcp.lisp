(deflookup
  euc-into-wcp
  ;reference columns
  (
    wcp.ARG_1_HI
    wcp.ARG_1_LO
    wcp.ARG_2_HI
    wcp.ARG_2_LO
    wcp.RES
    wcp.INST
  )
  ;source columns
  (
    0
    (* euc.REMAINDER euc.DONE)
    0
    (* euc.DIVISOR   euc.DONE)
    (* 1             euc.DONE)
    (* EVM_INST_LT   euc.DONE)
  ))


