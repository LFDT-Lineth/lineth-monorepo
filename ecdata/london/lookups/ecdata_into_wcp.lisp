(defun (ec_data-into-wcp-activation-flag)
  ecdata.WCP_FLAG)

(defclookup
  ecdata-into-wcp
  ;; target columns
  (
    wcp.ARG_1
    wcp.ARG_2
    wcp.RES
    wcp.INST
  )
  ;; source selector
  (ec_data-into-wcp-activation-flag)
  ;; source columns
  (
    (:: ecdata.WCP_ARG1_HI ecdata.WCP_ARG1_LO)
    (:: ecdata.WCP_ARG2_HI ecdata.WCP_ARG2_LO)
    ecdata.WCP_RES
    ecdata.WCP_INST
  ))


