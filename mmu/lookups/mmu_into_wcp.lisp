(defun (mmu-to-wcp-selector)
  (* mmu.PRPRC mmu.prprc/WCP_FLAG))

(defclookup
  mmu-into-wcp
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
  (mmu-to-wcp-selector)
  ;; source columns
  (
    mmu.prprc/WCP_ARG_1_HI
    mmu.prprc/WCP_ARG_1_LO
    0
    mmu.prprc/WCP_ARG_2_LO
    mmu.prprc/WCP_RES
    mmu.prprc/WCP_INST
  ))


