(defun (blockdata-into-wcp-selector)
  blockdata.WCP_FLAG)

(deflookup 
  blockdata-into-wcp
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
    (* blockdata.ARG_1_HI (blockdata-into-wcp-selector))
    (* blockdata.ARG_1_LO (blockdata-into-wcp-selector))
    (* blockdata.ARG_2_HI (blockdata-into-wcp-selector))
    (* blockdata.ARG_2_LO (blockdata-into-wcp-selector))
    (* blockdata.RES      (blockdata-into-wcp-selector))
    (* blockdata.EXO_INST (blockdata-into-wcp-selector))
  ))

