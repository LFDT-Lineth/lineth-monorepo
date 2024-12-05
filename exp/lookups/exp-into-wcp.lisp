(defun (exp-into-wcp-activation-flag) (* exp.PRPRC exp.preprocessing/WCP_FLAG))

(deflookup
  exp-into-wcp
  ;; target columns
  (
    wcp.ARGUMENT_1_HI
    wcp.ARGUMENT_1_LO
    wcp.ARGUMENT_2_HI
    wcp.ARGUMENT_2_LO
    wcp.RESULT
    wcp.INST
  )
  ;; source columns
  (
    (*   exp.preprocessing/WCP_ARG_1_HI   (exp-into-wcp-activation-flag))
    (*   exp.preprocessing/WCP_ARG_1_LO   (exp-into-wcp-activation-flag))
    (*   exp.preprocessing/WCP_ARG_2_HI   (exp-into-wcp-activation-flag))
    (*   exp.preprocessing/WCP_ARG_2_LO   (exp-into-wcp-activation-flag))
    (*   exp.preprocessing/WCP_RES        (exp-into-wcp-activation-flag))
    (*   exp.preprocessing/WCP_INST       (exp-into-wcp-activation-flag))
  ))


