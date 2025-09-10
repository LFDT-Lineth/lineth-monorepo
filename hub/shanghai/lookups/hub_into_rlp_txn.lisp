(defun (hub-into-rlp-txn-src-selector) hub.TX_WARM)

;; DUPLICATE
(defun (rlp-txn-depth-2)
  [rlptxn.DEPTH 2]) ;; ""

;; DUPLICATES
(defun (prewarming-phase-address-hi)     (+ (* hub.PEEK_AT_ACCOUNT hub.account/ADDRESS_HI) (* hub.PEEK_AT_STORAGE hub.storage/ADDRESS_HI)))
(defun (prewarming-phase-address-lo)     (+ (* hub.PEEK_AT_ACCOUNT hub.account/ADDRESS_LO) (* hub.PEEK_AT_STORAGE hub.storage/ADDRESS_LO)))
(defun (prewarming-phase-storage-key-hi) (* hub.PEEK_AT_STORAGE hub.storage/STORAGE_KEY_HI))
(defun (prewarming-phase-storage-key-lo) (* hub.PEEK_AT_STORAGE hub.storage/STORAGE_KEY_LO))

(defun (hub-into-rlp-txn-tgt-selector)   (*   rlptxn.REQUIRES_EVM_EXECUTION
                                              rlptxn.IS_PHASE_ACCESS_LIST
                                              (- 1 rlptxn.IS_PREFIX)))

(defclookup
  (hub-into-rlptxn :unchecked)
  ;; TODO: multiplication by selector likely unnecessary but as we
  ;; multiply by the same column for the lookup tlptxn into hub ...

  ;; target selector
  (hub-into-rlp-txn-tgt-selector)
  ;; target columns
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
  ;; source selector
  (hub-into-rlp-txn-src-selector)
  ;; source columns
  (
    1
    hub.ABSOLUTE_TRANSACTION_NUMBER
    hub.PEEK_AT_ACCOUNT
    hub.PEEK_AT_STORAGE

    (prewarming-phase-address-hi)
    (prewarming-phase-address-lo)
    (prewarming-phase-storage-key-hi)
    (prewarming-phase-storage-key-lo)
  ))
