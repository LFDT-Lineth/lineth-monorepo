(defun (blsdata-into-wcp-activation-flag)
  blsdata.WCP_FLAG)

(defclookup
  blsdata-into-wcp
  ; target columns
  (
    wcp.ARG_1
    wcp.ARG_2
    wcp.RES
    wcp.INST
  )
  ; source selector
  (blsdata-into-wcp-activation-flag)
  ; source columns
  (
    (:: blsdata.WCP_ARG1_HI blsdata.WCP_ARG1_LO)
    (:: blsdata.WCP_ARG2_HI blsdata.WCP_ARG2_LO)
    blsdata.WCP_RES
    blsdata.WCP_INST
  ))


