(defun   (txn-data---phase-sum)
     (+
       (*  txndata.RLP     RLP_RCPT_SUBPHASE_ID_TYPE        )
       (*  txndata.HUB     RLP_RCPT_SUBPHASE_ID_STATUS_CODE )
       (*  txndata.CMPTN   RLP_RCPT_SUBPHASE_ID_CUMUL_GAS   )
       ))

(defun   (txn-data---outgoing-value)
  (+
    (*  txndata.RLP     txndata.rlp/TX_TYPE     )
    (*  txndata.HUB     txndata.hub/STATUS_CODE )
    (*  txndata.CMPTN   txndata.GAS_CUMULATIVE  )
    ))



;; ""
(defclookup
  ( txndata-into-rlp-txn-rcpt   :unchecked )
  ; target selector
  ;
  ; target columns
  (
   rlptxrcpt.ABS_TX_NUM
   rlptxrcpt.PHASE_ID
   [rlptxrcpt.INPUT 1] ;; ""
   )
  ; source selector
  txndata.USER
  ; source columns
  (
   txndata.USER_TXN_NUMBER
   (txn-data---phase-sum)
   (txn-data---outgoing-value)
   )
  )

