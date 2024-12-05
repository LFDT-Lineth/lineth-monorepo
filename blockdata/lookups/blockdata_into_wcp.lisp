(deflookup
  blockdata-into-wcp
  ; target columns
  (
    wcp.ARGUMENT_1_HI
    wcp.ARGUMENT_1_LO
    wcp.ARGUMENT_2_HI
    wcp.ARGUMENT_2_LO
    wcp.RESULT
    wcp.INST
  )
  ; source columns
  (
    (* blockdata.WCP_FLAG blockdata.DATA_HI)
    (* blockdata.WCP_FLAG blockdata.DATA_LO)
    (* blockdata.WCP_FLAG (shift blockdata.DATA_HI -7)) ;; -7 = (- 0 (+ MAX_CT 1))

    (* blockdata.WCP_FLAG (shift blockdata.DATA_LO -7)) ;; -7 = (- 0 (+ MAX_CT 1))

    blockdata.WCP_FLAG
    (* blockdata.WCP_FLAG EVM_INST_GT)
  ))


