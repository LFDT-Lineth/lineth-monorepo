(defclookup
  (txndata-into-euc :unchecked)
  ;; target columns
  (
    euc.DIVIDEND
    euc.DIVISOR
    euc.QUOTIENT
  )
  ;; source selector
  txndata.EUC_FLAG
  ;; source columns
  (
    txndata.ARG_ONE_LO
    txndata.ARG_TWO_LO
    txndata.RES
  ))


