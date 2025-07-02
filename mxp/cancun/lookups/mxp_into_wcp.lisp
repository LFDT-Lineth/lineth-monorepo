(defun
  (mxp-to-wcp-selector)
  (* mxp.COMPUTATION mxp.computation/WCP_FLAG)
  )

(defclookup
  mxp-into-wcp
  ;; target columns
  (
    wcp.INST
    wcp.ARGUMENT_1_HI
    wcp.ARGUMENT_1_LO
    wcp.ARGUMENT_2_HI
    wcp.ARGUMENT_2_LO
    wcp.RESULT
  )
  ;; source selector
  (mxp-to-wcp-selector)  
  ;; source columns
  (
    mxp.computation/EXO_INST
    mxp.computation/ARG_1_HI
    mxp.computation/ARG_1_LO
    mxp.computation/ARG_2_HI
    mxp.computation/ARG_2_LO
    mxp.computation/RES_A
  ))
