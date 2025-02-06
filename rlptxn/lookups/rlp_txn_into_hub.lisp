(defun (rlp-txn-into-hub-src-selector) (* rlptxn.REQUIRES_EVM_EXECUTION rlptxn.IS_PHASE_ACCESS_LIST (- 1 rlptxn.IS_PREFIX)))

(deflookup 
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
  ;; source columns
  (
                                            (rlp-txn-into-hub-src-selector)
   (* rlptxn.ABS_TX_NUM                     (rlp-txn-into-hub-src-selector))
   (* (- 1 (rlp-txn-depth-2))               (rlp-txn-into-hub-src-selector))
   (* (rlp-txn-depth-2)                     (rlp-txn-into-hub-src-selector))
   (* rlptxn.ADDR_HI                        (rlp-txn-into-hub-src-selector))
   (* rlptxn.ADDR_LO                        (rlp-txn-into-hub-src-selector))
   (* [rlptxn.INPUT 1] (rlp-txn-depth-2)    (rlp-txn-into-hub-src-selector))
   (* [rlptxn.INPUT 2] (rlp-txn-depth-2)    (rlp-txn-into-hub-src-selector))
   )
  )


