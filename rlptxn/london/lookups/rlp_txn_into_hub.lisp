(defun (rlp-txn-into-hub-src-selector) (* rlptxn.REQUIRES_EVM_EXECUTION rlptxn.IS_PHASE_ACCESS_LIST (- 1 rlptxn.IS_PREFIX)))

(defclookup 
  rlptxn-into-hub
  ;; target columns
  (
   hub.TX_WARM
   hub.ABSOLUTE_TRANSACTION_NUMBER
   hub.PEEK_AT_ACCOUNT
   hub.PEEK_AT_STORAGE
   (prewarming-phase-address-hi)
   (prewarming-phase-address-lo)
   (prewarming-phase-storage-key-hi)
   (prewarming-phase-storage-key-lo)
   )
  ;; source selector
  (rlp-txn-into-hub-src-selector)
  ;; source columns
  (
   1
   rlptxn.ABS_TX_NUM
   (- 1 (rlp-txn-depth-2))
   (rlp-txn-depth-2)
   rlptxn.ADDR_HI
   rlptxn.ADDR_LO
   (* [rlptxn.INPUT 1] (rlp-txn-depth-2))
   (* [rlptxn.INPUT 2] (rlp-txn-depth-2))
   )
  )


