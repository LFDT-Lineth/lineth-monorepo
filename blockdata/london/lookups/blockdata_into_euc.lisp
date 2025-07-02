(defun (blockdata-into-euc-selector) blockdata.EUC_FLAG)

(defclookup 
  blockdata-into-euc
  ;; target columns
  (
    euc.IOMF
    euc.DIVIDEND
    euc.DIVISOR
    euc.QUOTIENT
  )
  ;; source selector
  (blockdata-into-euc-selector)
  ;; source columns
  (
    1
    blockdata.ARG_1_LO
    blockdata.ARG_2_LO
    blockdata.RES
  ))

