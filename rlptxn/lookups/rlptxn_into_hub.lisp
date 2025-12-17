(defun (sel-rlp-txn-into-hub) 
(force-bin 
   (* rlptxn.REQUIRES_EVM_EXECUTION 
   (+ rlptxn.IS_ACCESS_LIST_ADDRESS rlptxn.IS_ACCESS_LIST_STORAGE_KEY ) 
   (prev rlptxn.DONE))))

(defun (rlptxn---access-list---address-hi)          rlptxn.cmp/AUX_CCC_4)
(defun (rlptxn---access-list---address-lo)          rlptxn.cmp/AUX_CCC_5)

(defclookup 
  rlptxn-into-hub
  ;; target columns
  (
   hub.TX_WARM
   hub.USER_TXN_NUMBER
   hub.PEEK_AT_ACCOUNT
   hub.PEEK_AT_STORAGE
   (prewarming-phase-address-hi)
   (prewarming-phase-address-lo)
   (prewarming-phase-storage-key-hi)
   (prewarming-phase-storage-key-lo)
   )
  ;; source selector
  (sel-rlp-txn-into-hub)
  ;; source columns
  (
   1
   rlptxn.USER_TXN_NUMBER
   rlptxn.IS_ACCESS_LIST_ADDRESS
   rlptxn.IS_ACCESS_LIST_STORAGE_KEY
   (rlptxn---access-list---address-hi)
   (rlptxn---access-list---address-lo)
   (* rlptxn.cmp/EXO_DATA_1 rlptxn.IS_ACCESS_LIST_STORAGE_KEY)
   (* rlptxn.cmp/EXO_DATA_2 rlptxn.IS_ACCESS_LIST_STORAGE_KEY)
   )
  )


