(defun (blockdata-into-euc-selector) blockdata.EUC_FLAG)

(defclookup 
  (blockdata-into-euc :unchecked)
  ;; target columns
  (
    euc.DIVIDEND
    euc.DIVISOR
    euc.QUOTIENT
  )
  ;; source selector
  (blockdata-into-euc-selector)
  ;; source columns
  (
    blockdata.ARG_1_LO
    blockdata.ARG_2_LO
    blockdata.RES
  ))

