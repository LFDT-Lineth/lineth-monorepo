(defun (hub-into-rlp-txn-src-selector)
  hub.TX_WARM)

;; DUPLICATE
(defun (rlp-txn-depth-2)
  [rlptxn.DEPTH 2])

;; DUPLICATE
(defun (prewarming-phase-address-hi)
  (+ (* hub.PEEK_AT_ACCOUNT hub.account/ADDRESS_HI) (* hub.PEEK_AT_STORAGE hub.storage/ADDRESS_HI)))

;; DUPLICATE
(defun (prewarming-phase-address-lo)
  (+ (* hub.PEEK_AT_ACCOUNT hub.account/ADDRESS_LO) (* hub.PEEK_AT_STORAGE hub.storage/ADDRESS_LO)))

;; DUPLICATE
(defun (prewarming-phase-storage-key-hi)
  (* hub.PEEK_AT_STORAGE hub.storage/STORAGE_KEY_HI))

;; DUPLICATE
(defun (prewarming-phase-storage-key-lo)
  (* hub.PEEK_AT_STORAGE hub.storage/STORAGE_KEY_LO))

(defun (hub-into-rlp-txn-tgt-selector)
  (* (- 1 rlptxn.IS_PREFIX) rlptxn.IS_PHASE_ACCESS_LIST))

(deflookup 
  hub-into-rlp-txn
  ;; target columns
  (
    (* (hub-into-rlp-txn-tgt-selector) rlptxn.REQUIRES_EVM_EXECUTION)
    rlptxn.ABS_TX_NUM
    (* (- 1 (rlp-txn-depth-2)) (hub-into-rlp-txn-tgt-selector))
    (* (rlp-txn-depth-2) (hub-into-rlp-txn-tgt-selector)) ;; TODO: multiplication by selector likely unnecessary

    rlptxn.ADDR_HI
    rlptxn.ADDR_LO
    (* [rlptxn.INPUT 1] (rlp-txn-depth-2))
    (* [rlptxn.INPUT 2] (rlp-txn-depth-2))
  )
  ;; source columns
  (
    (hub-into-rlp-txn-src-selector)
    (* hub.ABSOLUTE_TRANSACTION_NUMBER (hub-into-rlp-txn-src-selector))
    (* hub.PEEK_AT_ACCOUNT (hub-into-rlp-txn-src-selector))
    (* hub.PEEK_AT_STORAGE (hub-into-rlp-txn-src-selector))
    (* (prewarming-phase-address-hi) (hub-into-rlp-txn-src-selector))
    (* (prewarming-phase-address-lo) (hub-into-rlp-txn-src-selector))
    (* (prewarming-phase-storage-key-hi) (hub-into-rlp-txn-src-selector))
    (* (prewarming-phase-storage-key-lo) (hub-into-rlp-txn-src-selector))
  ))


