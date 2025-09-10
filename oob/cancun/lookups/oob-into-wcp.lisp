(defun (oob-into-wcp-activation-flag)
  oob.WCP_FLAG)

(defclookup
  (oob-into-wcp :unchecked)
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
  (oob-into-wcp-activation-flag)
  ;; source columns
  (
    [oob.OUTGOING_DATA 1]
    [oob.OUTGOING_DATA 2]
    [oob.OUTGOING_DATA 3]
    [oob.OUTGOING_DATA 4]
    oob.OUTGOING_RES_LO
    oob.OUTGOING_INST
  ))


