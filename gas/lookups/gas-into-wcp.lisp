(defun (gas-into-wcp-activation-flag)
  gas.IOMF)

(deflookup 
  gas-into-wcp
  (
    wcp.ARG_1_HI
    wcp.ARG_1_LO
    wcp.ARG_2_HI
    wcp.ARG_2_LO
    wcp.RES
    wcp.INST
  )
  (
    0
    (* gas.WCP_ARG1_LO (wcp-activation-flag))
    0
    (* gas.WCP_ARG2_LO (wcp-activation-flag))
    (* gas.WCP_RES (wcp-activation-flag))
    (* gas.WCP_INST (wcp-activation-flag))
  ))


