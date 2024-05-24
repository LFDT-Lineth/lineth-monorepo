(defun (wcp-activation-flag)
  oob.WCP_FLAG)

(deflookup 
  oob-into-wcp
  (
    wcp.ARGUMENT_1_HI
    wcp.ARGUMENT_1_LO
    wcp.ARGUMENT_2_HI
    wcp.ARGUMENT_2_LO
    wcp.RESULT
    wcp.INST
  )
  (
    (* [oob.OUTGOING_DATA 1] (wcp-activation-flag))
    (* [oob.OUTGOING_DATA 2] (wcp-activation-flag))
    (* [oob.OUTGOING_DATA 3] (wcp-activation-flag))
    (* [oob.OUTGOING_DATA 4] (wcp-activation-flag))
    (* oob.OUTGOING_RES_LO (wcp-activation-flag))
    (* oob.OUTGOING_INST (wcp-activation-flag))
  ))


