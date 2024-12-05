(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;   X Finalization phase   ;;
;;   X.1 Introduction       ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconst
  ;; transaction success
  ;; offsets:
  row-offset---tx-finl---success---sender-account-row      0
  row-offset---tx-finl---success---coinbase-account-row    1
  row-offset---tx-finl---success---transaction-row         2
  ;; number of rows
  tx-finl---success---number-of-rows                       3

  ;; transaction failure
  ;; offsets:
  row-offset---tx-finl---failure---sender-account-row      0
  row-offset---tx-finl---failure---recipient-account-row   1
  row-offset---tx-finl---failure---coinbase-account-row    2
  row-offset---tx-finl---failure---transaction-row         3
  ;; number of rows
  tx-finl---failure---number-of-rows                       4
  )


;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;   X.2 Peeking flags   ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (tx-finalization---precondition)         (*    (prev TX_EXEC)    TX_FINL))

(defconstraint   tx-finalization---setting-peeking-flags (:guard (tx-finalization---precondition))
                 (if-zero   (prev CONTEXT_WILL_REVERT)
                            ;; cn_will_rev ≡ 0
                            (eq!    tx-finl---success---number-of-rows
                                    (+   (shift PEEK_AT_ACCOUNT        row-offset---tx-finl---success---sender-account-row    )
                                         (shift PEEK_AT_ACCOUNT        row-offset---tx-finl---success---coinbase-account-row  )
                                         (shift PEEK_AT_TRANSACTION    row-offset---tx-finl---success---transaction-row       )))
                            ;; cn_will_rev ≡ 1
                            (eq!    tx-finl---failure---number-of-rows
                                    (+   (shift PEEK_AT_ACCOUNT        row-offset---tx-finl---failure---sender-account-row    )
                                         (shift PEEK_AT_ACCOUNT        row-offset---tx-finl---failure---recipient-account-row )
                                         (shift PEEK_AT_ACCOUNT        row-offset---tx-finl---failure---coinbase-account-row  )
                                         (shift PEEK_AT_TRANSACTION    row-offset---tx-finl---failure---transaction-row       )))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;   X.3 Transaction SUCCESS   ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (tx-finalization---success---precondition) (* (tx-finalization---precondition) (prev (- 1 CONTEXT_WILL_REVERT))))

;; sender account
;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;
(defconstraint   tx-finalization---success---setting-sender-account-row                        (:guard (tx-finalization---success---precondition))
                 (begin
                   (eq!     (shift account/ADDRESS_HI             row-offset---tx-finl---success---sender-account-row)     (shift    transaction/FROM_ADDRESS_HI    row-offset---tx-finl---success---transaction-row))
                   (eq!     (shift account/ADDRESS_LO             row-offset---tx-finl---success---sender-account-row)     (shift    transaction/FROM_ADDRESS_LO    row-offset---tx-finl---success---transaction-row))
                   (account-increment-balance-by                  row-offset---tx-finl---success---sender-account-row      (tx-finalization---success---sender-refund))
                   (account-same-nonce                            row-offset---tx-finl---success---sender-account-row)
                   (account-same-code                             row-offset---tx-finl---success---sender-account-row)
                   (account-same-deployment-number-and-status     row-offset---tx-finl---success---sender-account-row)
                   (account-same-warmth                           row-offset---tx-finl---success---sender-account-row)
                   (account-same-marked-for-selfdestruct          row-offset---tx-finl---success---sender-account-row)
                   (DOM-SUB-stamps---standard                     row-offset---tx-finl---success---sender-account-row
                                                                  0)))

(defun (tx-finalization---success---sender-refund)    (* (shift   transaction/GAS_PRICE           row-offset---tx-finl---success---transaction-row)
                                                         (shift   transaction/REFUND_EFFECTIVE    row-offset---tx-finl---success---transaction-row)))

;; coinbase address
;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;
(defconstraint   tx-finalization---success---setting-coinbase-account-row                      (:guard (tx-finalization---success---precondition))
                 (begin
                   (eq!     (shift account/ADDRESS_HI             row-offset---tx-finl---success---coinbase-account-row)     (shift    transaction/COINBASE_ADDRESS_HI    row-offset---tx-finl---success---transaction-row))
                   (eq!     (shift account/ADDRESS_LO             row-offset---tx-finl---success---coinbase-account-row)     (shift    transaction/COINBASE_ADDRESS_LO    row-offset---tx-finl---success---transaction-row))
                   (account-increment-balance-by                  row-offset---tx-finl---success---coinbase-account-row      (tx-finalization---success---coinbase-reward))
                   (account-same-nonce                            row-offset---tx-finl---success---coinbase-account-row)
                   (account-same-code                             row-offset---tx-finl---success---coinbase-account-row)
                   (account-same-deployment-number-and-status     row-offset---tx-finl---success---coinbase-account-row)
                   (account-same-warmth                           row-offset---tx-finl---success---coinbase-account-row)
                   (account-same-marked-for-selfdestruct          row-offset---tx-finl---success---coinbase-account-row)
                   (DOM-SUB-stamps---standard                     row-offset---tx-finl---success---coinbase-account-row
                                                                  1)))

(defun    (tx-finalization---success---coinbase-reward)    (shift    (*    transaction/PRIORITY_FEE_PER_GAS    (-    transaction/GAS_LIMIT    transaction/REFUND_EFFECTIVE))
                                                                     row-offset---tx-finl---success---transaction-row))

;; justifying TXN_DATA predictions
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint   tx-finalization---success---justifying-txn-data-prediction---status-code
                 (:guard (tx-finalization---success---precondition))
                 (eq!   (shift transaction/STATUS_CODE               row-offset---tx-finl---success---transaction-row)   1))

(defconstraint   tx-finalization---success---justifying-txn-data-prediction---refund-counter
                 (:guard (tx-finalization---success---precondition))
                 (eq!   (shift transaction/REFUND_COUNTER_INFINITY   row-offset---tx-finl---success---transaction-row)   (shift   REFUND_COUNTER   -1)))

(defconstraint   tx-finalization---success---justifying-txn-data-prediction---leftover-gas
                 (:guard (tx-finalization---success---precondition))
                 (eq!   (shift transaction/GAS_LEFTOVER              row-offset---tx-finl---success---transaction-row)   (shift   GAS_NEXT         -1)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;   X.3 Transaction FAILURE   ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (tx-finalization---failure---precondition) (* (tx-finalization---precondition) (prev CONTEXT_WILL_REVERT)))

;; sender account
;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;
(defconstraint   tx-finalization---failure---setting-sender-account-row                        (:guard (tx-finalization---failure---precondition))
                 (begin
                   (eq!     (shift account/ADDRESS_HI             row-offset---tx-finl---failure---sender-account-row)     (shift    transaction/FROM_ADDRESS_HI    row-offset---tx-finl---failure---transaction-row))
                   (eq!     (shift account/ADDRESS_LO             row-offset---tx-finl---failure---sender-account-row)     (shift    transaction/FROM_ADDRESS_LO    row-offset---tx-finl---failure---transaction-row))
                   (account-increment-balance-by                  row-offset---tx-finl---failure---sender-account-row      (tx-finalization---failure---sender-refund))
                   (account-same-nonce                            row-offset---tx-finl---failure---sender-account-row)
                   (account-same-code                             row-offset---tx-finl---failure---sender-account-row)
                   (account-same-deployment-number-and-status     row-offset---tx-finl---failure---sender-account-row)
                   (account-same-warmth                           row-offset---tx-finl---failure---sender-account-row)
                   (account-same-marked-for-selfdestruct          row-offset---tx-finl---failure---sender-account-row)
                   (DOM-SUB-stamps---standard                     row-offset---tx-finl---failure---sender-account-row
                                                                  0)))

(defun (tx-finalization---failure---sender-refund)    (+  (* (shift   transaction/GAS_PRICE           row-offset---tx-finl---failure---transaction-row)
                                                             (shift   transaction/REFUND_EFFECTIVE    row-offset---tx-finl---failure---transaction-row))
                                                          (shift      transaction/VALUE               row-offset---tx-finl---failure---transaction-row)))

;; recipient account
;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;
(defconstraint   tx-finalization---failure---setting-recipient-account-row                        (:guard (tx-finalization---failure---precondition))
                 (begin
                   (eq!     (shift account/ADDRESS_HI             row-offset---tx-finl---failure---recipient-account-row)     (shift    transaction/TO_ADDRESS_HI    row-offset---tx-finl---failure---transaction-row))
                   (eq!     (shift account/ADDRESS_LO             row-offset---tx-finl---failure---recipient-account-row)     (shift    transaction/TO_ADDRESS_LO    row-offset---tx-finl---failure---transaction-row))
                   (account-decrement-balance-by                  row-offset---tx-finl---failure---recipient-account-row      (shift    transaction/VALUE            row-offset---tx-finl---failure---transaction-row))
                   (account-same-nonce                            row-offset---tx-finl---failure---recipient-account-row)
                   (account-same-code                             row-offset---tx-finl---failure---recipient-account-row)
                   (account-same-deployment-number-and-status     row-offset---tx-finl---failure---recipient-account-row)
                   (account-same-warmth                           row-offset---tx-finl---failure---recipient-account-row)
                   (account-same-marked-for-selfdestruct          row-offset---tx-finl---failure---recipient-account-row)
                   (DOM-SUB-stamps---standard                     row-offset---tx-finl---failure---recipient-account-row
                                                                  1)))

;; coinbase address
;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;
(defconstraint   tx-finalization---failure---setting-coinbase-account-row                      (:guard (tx-finalization---failure---precondition))
                 (begin
                   (eq!     (shift account/ADDRESS_HI             row-offset---tx-finl---failure---coinbase-account-row)     (shift    transaction/COINBASE_ADDRESS_HI    row-offset---tx-finl---failure---transaction-row))
                   (eq!     (shift account/ADDRESS_LO             row-offset---tx-finl---failure---coinbase-account-row)     (shift    transaction/COINBASE_ADDRESS_LO    row-offset---tx-finl---failure---transaction-row))
                   (account-increment-balance-by                  row-offset---tx-finl---failure---coinbase-account-row      (tx-finalization---failure---coinbase-reward))
                   (account-same-nonce                            row-offset---tx-finl---failure---coinbase-account-row)
                   (account-same-code                             row-offset---tx-finl---failure---coinbase-account-row)
                   (account-same-deployment-number-and-status     row-offset---tx-finl---failure---coinbase-account-row)
                   (account-same-warmth                           row-offset---tx-finl---failure---coinbase-account-row)
                   (account-same-marked-for-selfdestruct          row-offset---tx-finl---failure---coinbase-account-row)
                   (DOM-SUB-stamps---standard                     row-offset---tx-finl---failure---coinbase-account-row
                                                                  2)))

(defun    (tx-finalization---failure---coinbase-reward)    (shift    (*    transaction/PRIORITY_FEE_PER_GAS    (-    transaction/GAS_LIMIT    transaction/REFUND_EFFECTIVE))
                                                                     row-offset---tx-finl---failure---transaction-row))

;; justifying TXN_DATA predictions
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   tx-finalization---failure---justifying-txn-data-prediction---status-code
                 (:guard (tx-finalization---failure---precondition))
                 (eq!   (shift transaction/STATUS_CODE               row-offset---tx-finl---failure---transaction-row)   0))

(defconstraint   tx-finalization---failure---justifying-txn-data-prediction---refund-counter
                 (:guard (tx-finalization---failure---precondition))
                 (eq!   (shift transaction/REFUND_COUNTER_INFINITY   row-offset---tx-finl---failure---transaction-row)   (shift   REFUND_COUNTER   -1)))

(defconstraint   tx-finalization---failure---justifying-txn-data-prediction---leftover-gas
                 (:guard (tx-finalization---failure---precondition))
                 (eq!   (shift transaction/GAS_LEFTOVER              row-offset---tx-finl---failure---transaction-row)   (shift   GAS_NEXT         -1)))
