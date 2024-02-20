(defun (wcp-activation-flag)
  (* exp.PRPRC exp.preprocessing/WCP_FLAG))

(deflookup 
  exp-into-wcp
  (
    wcp.ARGUMENT_1_HI
    wcp.ARGUMENT_1_LO
    wcp.ARGUMENT_2_HI
    wcp.ARGUMENT_2_LO
    wcp.RESULT
    wcp.INST
  )
  (
    (* exp.preprocessing/WCP_ARG_1_HI (wcp-activation-flag))
    (* exp.preprocessing/WCP_ARG_1_LO (wcp-activation-flag))
    (* exp.preprocessing/WCP_ARG_2_HI (wcp-activation-flag))
    (* exp.preprocessing/WCP_ARG_2_LO (wcp-activation-flag))
    (* exp.preprocessing/WCP_RES (wcp-activation-flag))
    (* exp.preprocessing/WCP_INST (wcp-activation-flag))
  ))


