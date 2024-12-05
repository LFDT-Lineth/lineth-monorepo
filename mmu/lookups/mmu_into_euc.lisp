(defun (mmu-to-euc-selector)
  (* mmu.PRPRC mmu.prprc/EUC_FLAG))

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
    (* mmu.prprc/EUC_A (mmu-to-euc-selector))
    (* mmu.prprc/EUC_B (mmu-to-euc-selector))
    (* mmu.prprc/EUC_QUOT (mmu-to-euc-selector))
    (* mmu.prprc/EUC_REM (mmu-to-euc-selector))
    (* mmu.prprc/EUC_CEIL (mmu-to-euc-selector))
    (mmu-to-euc-selector)
  ))


