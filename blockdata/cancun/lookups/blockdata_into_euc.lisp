(defun (blockdata-into-euc-selector) blockdata.EUC_FLAG)

(deflookup 
  blockdata-into-euc
  ;; target columns
  (
    euc.IOMF
    euc.DIVIDEND
    euc.DIVISOR
    euc.QUOTIENT
  )
  ;; source columns
  (
    (* 1                  (blockdata-into-euc-selector))
    (* blockdata.ARG_1_LO (blockdata-into-euc-selector))
    (* blockdata.ARG_2_LO (blockdata-into-euc-selector))
    (* blockdata.RES      (blockdata-into-euc-selector))
  ))

