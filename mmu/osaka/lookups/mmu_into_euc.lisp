(defun (mmu-to-euc-selector)
  (* mmu.PRPRC mmu.prprc/EUC_FLAG))

(defclookup
  mmu-into-euc
  ;;target columns
  (
    euc.DIVIDEND
    euc.DIVISOR
    euc.QUOTIENT
    euc.REMAINDER
    euc.CEIL
  )
  ;; source selector
  (mmu-to-euc-selector)
  ;; source columns
  (
    mmu.prprc/EUC_A
    mmu.prprc/EUC_B
    mmu.prprc/EUC_QUOT
    mmu.prprc/EUC_REM
    mmu.prprc/EUC_CEIL
  ))


