(defun
  (mxp-to-wcp-selector)
  (* mxp.COMPUTATION mxp.computation/WCP_FLAG)
  )

(defclookup
  (mxp-into-wcp :unchecked)
  ;; target columns
  (
    wcp.INST
    wcp.ARG_1
    wcp.ARG_2
    wcp.RES
  )
  ;; source selector
  (mxp-to-wcp-selector)  
  ;; source columns
  (
    mxp.computation/EXO_INST
    (:: mxp.computation/ARG_1_HI mxp.computation/ARG_1_LO)
    (:: mxp.computation/ARG_2_HI mxp.computation/ARG_2_LO)
    mxp.computation/RES_A
  ))
