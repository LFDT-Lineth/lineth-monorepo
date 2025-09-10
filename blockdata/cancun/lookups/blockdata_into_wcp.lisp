(defun (blockdata-into-wcp-selector)
  blockdata.WCP_FLAG)

(defclookup 
  (blockdata-into-wcp :unchecked)
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
  (blockdata-into-wcp-selector)
  ;; source columns
  (
    blockdata.ARG_1_HI
    blockdata.ARG_1_LO
    blockdata.ARG_2_HI
    blockdata.ARG_2_LO
    blockdata.RES
    blockdata.EXO_INST
  ))

