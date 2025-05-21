(defun (oob-into-wcp-activation-flag)
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
    (* [oob.OUTGOING_DATA 1] (oob-into-wcp-activation-flag))
    (* [oob.OUTGOING_DATA 2] (oob-into-wcp-activation-flag))
    (* [oob.OUTGOING_DATA 3] (oob-into-wcp-activation-flag))
    (* [oob.OUTGOING_DATA 4] (oob-into-wcp-activation-flag))
    (* oob.OUTGOING_RES_LO (oob-into-wcp-activation-flag))
    (* oob.OUTGOING_INST (oob-into-wcp-activation-flag))
  ))


