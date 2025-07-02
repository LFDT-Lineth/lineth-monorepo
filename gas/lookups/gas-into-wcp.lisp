(defun (gas-into-wcp-activation-flag)
  gas.IOMF)

(defclookup
    gas-into-wcp
  ;; target columns
  (
    wcp.ARG_1_HI
    wcp.ARG_1_LO
    wcp.ARG_2_HI
    wcp.ARG_2_LO
    wcp.RES
    wcp.INST
  )
  ;; source selector
  (gas-into-wcp-activation-flag)
  ;; source columns
  (
    0
    gas.WCP_ARG1_LO
    0
    gas.WCP_ARG2_LO
    gas.WCP_RES
    gas.WCP_INST
  ))


