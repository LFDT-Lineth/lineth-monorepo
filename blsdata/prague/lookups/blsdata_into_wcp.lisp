(defun (blsdata-into-wcp-activation-flag)
  blsdata.WCP_FLAG)

(deflookup
  blsdata-into-wcp
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
    (* blsdata.WCP_ARG1_HI (blsdata-into-wcp-activation-flag))
    (* blsdata.WCP_ARG1_LO (blsdata-into-wcp-activation-flag))
    (* blsdata.WCP_ARG2_HI (blsdata-into-wcp-activation-flag))
    (* blsdata.WCP_ARG2_LO (blsdata-into-wcp-activation-flag))
    (* blsdata.WCP_RES (blsdata-into-wcp-activation-flag))
    (* blsdata.WCP_INST (blsdata-into-wcp-activation-flag))
  ))


