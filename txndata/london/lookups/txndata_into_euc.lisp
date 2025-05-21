(deflookup
  txndata-into-euc
  ; target columns
  (
    euc.DONE
    euc.DIVIDEND
    euc.DIVISOR
    euc.QUOTIENT
  )
  ; source columns
  (
    txndata.EUC_FLAG
    (* txndata.EUC_FLAG txndata.ARG_ONE_LO)
    (* txndata.EUC_FLAG txndata.ARG_TWO_LO)
    (* txndata.EUC_FLAG txndata.RES)
  ))


