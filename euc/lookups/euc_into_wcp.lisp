(defclookup
  euc-into-wcp
  ;; target columns
  (
    wcp.ARG_1_HI
    wcp.ARG_1_LO
    wcp.ARG_2_HI
    wcp.ARG_2_LO
    wcp.RES
    wcp.INST
  )
  ;; source selector
  euc.DONE
  ;; source columns
  (
    0
    euc.REMAINDER
    0
    euc.DIVISOR
    1
    EVM_INST_LT
  ))


