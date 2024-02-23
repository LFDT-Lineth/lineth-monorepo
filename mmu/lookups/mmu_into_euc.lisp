(deflookup 
  mmu-into-euc
  ;reference columns
  (
    euc.DIVIDEND
    euc.DIVISOR
    euc.QUOTIENT
    euc.REMAINDER
    euc.CEIL
    euc.DONE
  )
  ;source columns
  (
    (* mmu.prprc/EUC_A mmu.prprc/EUC_FLAG)
    (* mmu.prprc/EUC_B mmu.prprc/EUC_FLAG)
    (* mmu.prprc/EUC_QUOT mmu.prprc/EUC_FLAG)
    (* mmu.prprc/EUC_REM mmu.prprc/EUC_FLAG)
    (* mmu.prprc/EUC_CEIL mmu.prprc/EUC_FLAG)
    mmu.prprc/EUC_FLAG
  ))


