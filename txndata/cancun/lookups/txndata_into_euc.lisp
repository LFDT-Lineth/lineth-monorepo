(defun   (txn-data-into-euc-selector)   (* txndata.CMPTN txndata.computation/EUC_FLAG))

(defclookup
  txndata-into-euc
  ; target columns
  (
   euc.DONE
   euc.DIVIDEND
   euc.DIVISOR
   euc.QUOTIENT
   euc.REMAINDER
   )
  ; source selector
  (txn-data-into-euc-selector)
  ; source columns
  (
   1
   txndata.computation/ARG_1_LO
   txndata.computation/ARG_2_LO
   txndata.computation/EUC_QUOTIENT
   txndata.computation/EUC_REMAINDER
   )
  )

