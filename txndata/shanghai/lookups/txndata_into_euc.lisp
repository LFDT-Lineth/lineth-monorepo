(defclookup
  (txndata-into-euc :unchecked)
  ;; target columns
  (
    euc.DONE
    euc.DIVIDEND
    euc.DIVISOR
    euc.QUOTIENT
  )
  ;; source selector
  txndata.EUC_FLAG
  ;; source columns
  (
    1
    txndata.ARG_ONE_LO
    txndata.ARG_TWO_LO
    txndata.RES
  ))


