(deflookup 
  mmu-into-wcp
  ;reference columns
  (
    wcp.ARG_1_HI
    wcp.ARG_1_LO
    wcp.ARG_2_HI
    wcp.ARG_2_LO
    wcp.RES
    wcp.INST
  )
  ;source columns
  (
    (* mmu.prprc/WCP_ARG_1_HI mmu.prprc/WCP_FLAG)
    (* mmu.prprc/WCP_ARG_1_LO mmu.prprc/WCP_FLAG)
    0
    (* mmu.prprc/WCP_ARG_2_LO mmu.prprc/WCP_FLAG)
    (* mmu.prprc/WCP_RES mmu.prprc/WCP_FLAG)
    (* mmu.prprc/WCP_INST mmu.prprc/WCP_FLAG)
  ))


