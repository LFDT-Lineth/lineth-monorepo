(defun (hub-into-rlp-txn-src-selector) hub.TX_WARM)

(defun (prewarming-phase-address-hi)     (+ (* hub.PEEK_AT_ACCOUNT hub.account/ADDRESS_HI) (* hub.PEEK_AT_STORAGE hub.storage/ADDRESS_HI)))
(defun (prewarming-phase-address-lo)     (+ (* hub.PEEK_AT_ACCOUNT hub.account/ADDRESS_LO) (* hub.PEEK_AT_STORAGE hub.storage/ADDRESS_LO)))
(defun (prewarming-phase-storage-key-hi) (* hub.PEEK_AT_STORAGE hub.storage/STORAGE_KEY_HI))
(defun (prewarming-phase-storage-key-lo) (* hub.PEEK_AT_STORAGE hub.storage/STORAGE_KEY_LO))

(defun (hub-into-rlp-txn-tgt-selector)   (sel-rlp-txn-into-hub))

(defclookup
  (hub-into-rlptxn :unchecked)

  ;; target selector
  (hub-into-rlp-txn-tgt-selector)
  ;; target columns  
  (
  rlptxn.USER_TXN_NUMBER
   rlptxn.IS_ACCESS_LIST_ADDRESS
   rlptxn.IS_ACCESS_LIST_STORAGE_KEY
   (rlptxn---access-list---address-hi)
   (rlptxn---access-list---address-lo)
   (* rlptxn.cmp/EXO_DATA_1 rlptxn.IS_ACCESS_LIST_STORAGE_KEY)
   (* rlptxn.cmp/EXO_DATA_2 rlptxn.IS_ACCESS_LIST_STORAGE_KEY)
  )
  ;; source selector
  (hub-into-rlp-txn-src-selector)
  ;; source columns
  (
    hub.USER_TXN_NUMBER
    hub.PEEK_AT_ACCOUNT
    hub.PEEK_AT_STORAGE

    (prewarming-phase-address-hi)
    (prewarming-phase-address-lo)
    (prewarming-phase-storage-key-hi)
    (prewarming-phase-storage-key-lo)
  ))
