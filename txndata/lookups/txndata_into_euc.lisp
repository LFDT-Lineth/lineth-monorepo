(deflookup 
  txn_data_into_euc
  ; target columns
  (
    euc.DONE
    euc.DIVIDEND
    euc.DIVISOR
    euc.QUOTIENT
  )
  ; source columns
  (
    txnData.EUC_FLAG
    (* txnData.EUC_FLAG txnData.ARG_ONE_LO)
    (* txnData.EUC_FLAG txnData.ARG_TWO_LO)
    (* txnData.EUC_FLAG txnData.RES)
  ))


