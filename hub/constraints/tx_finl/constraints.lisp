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
  tx-finl---success---row-offset---sender-account-row      0
  tx-finl---success---row-offset---coinbase-account-row    1
  tx-finl---success---row-offset---transaction-row         2
  ;; number of rows
  tx-finl---success---number-of-rows                       3

  ;; transaction failure
  ;; offsets:
  tx-finl---failure---row-offset---sender-account-row      0
  tx-finl---failure---row-offset---recipient-account-row   1
  tx-finl---failure---row-offset---coinbase-account-row    2
  tx-finl---failure---row-offset---transaction-row         3
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
                                    (+   (shift PEEK_AT_ACCOUNT        tx-finl---success---row-offset---sender-account-row    )
                                         (shift PEEK_AT_ACCOUNT        tx-finl---success---row-offset---coinbase-account-row  )
                                         (shift PEEK_AT_TRANSACTION    tx-finl---success---row-offset---transaction-row       )))
                            ;; cn_will_rev ≡ 1
                            (eq!    tx-finl---failure---number-of-rows
                                    (+   (shift PEEK_AT_ACCOUNT        tx-finl---failure---row-offset---sender-account-row    )
                                         (shift PEEK_AT_ACCOUNT        tx-finl---failure---row-offset---recipient-account-row )
                                         (shift PEEK_AT_ACCOUNT        tx-finl---failure---row-offset---coinbase-account-row  )
                                         (shift PEEK_AT_TRANSACTION    tx-finl---failure---row-offset---transaction-row       )))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;   X.3 Transaction SUCCESS   ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (tx-finalization---success-precondition) (* (tx-finalization---precondition) (prev (- 1 CONTEXT_WILL_REVERT))))

;; sender account
;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;
(defconstraint   tx-finalization---success-setting-sender-account-row                        (:guard (tx-finalization---success-precondition))
                 (begin
                   (eq!     (shift account/ADDRESS_HI             tx-finl---success---row-offset---sender-account-row)     (shift    transaction/FROM_ADDRESS_HI    tx-finl---success---row-offset---transaction-row))
                   (eq!     (shift account/ADDRESS_LO             tx-finl---success---row-offset---sender-account-row)     (shift    transaction/FROM_ADDRESS_LO    tx-finl---success---row-offset---transaction-row))
                   (account-increment-balance-by                  tx-finl---success---row-offset---sender-account-row      (tx-finalization---success-sender-refund))
                   (account-same-nonce                            tx-finl---success---row-offset---sender-account-row)
                   (account-same-code                             tx-finl---success---row-offset---sender-account-row)
                   (account-same-deployment-number-and-status     tx-finl---success---row-offset---sender-account-row)
                   (account-same-warmth                           tx-finl---success---row-offset---sender-account-row)
                   (account-same-marked-for-selfdestruct          tx-finl---success---row-offset---sender-account-row)
                   (DOM-SUB-stamps---standard                     tx-finl---success---row-offset---sender-account-row
                                                                  0)))

(defun (tx-finalization---success-sender-refund)    (* (shift   transaction/GAS_PRICE           tx-finl---success---row-offset---transaction-row)
                                                       (shift   transaction/REFUND_EFFECTIVE    tx-finl---success---row-offset---transaction-row)))

;; coinbase address
;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;
(defconstraint   tx-finalization---success-setting-coinbase-account-row                      (:guard (tx-finalization---success-precondition))
                 (begin
                   (eq!     (shift account/ADDRESS_HI             tx-finl---success---row-offset---coinbase-account-row)     (shift    transaction/COINBASE_ADDRESS_HI    tx-finl---success---row-offset---transaction-row))
                   (eq!     (shift account/ADDRESS_LO             tx-finl---success---row-offset---coinbase-account-row)     (shift    transaction/COINBASE_ADDRESS_LO    tx-finl---success---row-offset---transaction-row))
                   (account-increment-balance-by                  tx-finl---success---row-offset---coinbase-account-row      (tx-finalization---success-coinbase-reward))
                   (account-same-nonce                            tx-finl---success---row-offset---coinbase-account-row)
                   (account-same-code                             tx-finl---success---row-offset---coinbase-account-row)
                   (account-same-deployment-number-and-status     tx-finl---success---row-offset---coinbase-account-row)
                   (account-same-warmth                           tx-finl---success---row-offset---coinbase-account-row)
                   (account-same-marked-for-selfdestruct          tx-finl---success---row-offset---coinbase-account-row)
                   (DOM-SUB-stamps---standard                     tx-finl---success---row-offset---coinbase-account-row
                                                                  1)))

;; (defun (tx-finalization---success-coinbase-reward)
;;   (if-zero   (force-bin   (shift    transaction/IS_TYPE2          tx-finl---success---row-offset---transaction-row))
;;              ;; TYPE 0 / TYPE 1
;;              (* (shift      transaction/GAS_PRICE                 tx-finl---success---row-offset---transaction-row)
;;                 (- (shift   transaction/GAS_LIMIT                 tx-finl---success---row-offset---transaction-row)
;;                    (shift   transaction/REFUND_EFFECTIVE          tx-finl---success---row-offset---transaction-row)))
;;              ;; TYPE 2
;;              (* (- (shift   transaction/GAS_PRICE                 tx-finl---success---row-offset---transaction-row)
;;                    (shift   transaction/BASEFEE                   tx-finl---success---row-offset---transaction-row))
;;                 (- (shift   transaction/GAS_LIMIT                 tx-finl---success---row-offset---transaction-row)
;;                    (shift   transaction/REFUND_EFFECTIVE          tx-finl---success---row-offset---transaction-row)))))


(defun    (tx-finalization---success-coinbase-reward)    (shift    (*    transaction/PRIORITY_FEE_PER_GAS    (-    transaction/GAS_LIMIT    transaction/REFUND_EFFECTIVE))
                                                                   tx-finl---success---row-offset---transaction-row))

;; justifying TXN_DATA predictions
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint   tx-finalization---success-justifying-txn-data-prediction            (:guard (tx-finalization---success-precondition))
                 (begin
                   (eq!   (shift transaction/STATUS_CODE               tx-finl---success---row-offset---transaction-row)   1)
                   (eq!   (shift transaction/REFUND_COUNTER_INFINITY   tx-finl---success---row-offset---transaction-row)   (shift   REFUND_COUNTER   -1))
                   (eq!   (shift transaction/GAS_LEFTOVER              tx-finl---success---row-offset---transaction-row)   (shift   GAS_NEXT         -1))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;   X.3 Transaction FAILURE   ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (tx-finalization---failure-precondition) (* (tx-finalization---precondition) (prev CONTEXT_WILL_REVERT)))

;; sender account
;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;
(defconstraint   tx-finalization---failure-setting-sender-account-row                        (:guard (tx-finalization---failure-precondition))
                 (begin
                   (eq!     (shift account/ADDRESS_HI             tx-finl---failure---row-offset---sender-account-row)     (shift    transaction/FROM_ADDRESS_HI    tx-finl---failure---row-offset---transaction-row))
                   (eq!     (shift account/ADDRESS_LO             tx-finl---failure---row-offset---sender-account-row)     (shift    transaction/FROM_ADDRESS_LO    tx-finl---failure---row-offset---transaction-row))
                   (account-increment-balance-by                  tx-finl---failure---row-offset---sender-account-row      (tx-finalization---failure-sender-refund))
                   (account-same-nonce                            tx-finl---failure---row-offset---sender-account-row)
                   (account-same-code                             tx-finl---failure---row-offset---sender-account-row)
                   (account-same-deployment-number-and-status     tx-finl---failure---row-offset---sender-account-row)
                   (account-same-warmth                           tx-finl---failure---row-offset---sender-account-row)
                   (account-same-marked-for-selfdestruct          tx-finl---failure---row-offset---sender-account-row)
                   (DOM-SUB-stamps---standard                     tx-finl---failure---row-offset---sender-account-row
                                                                  0)))

(defun (tx-finalization---failure-sender-refund)    (+  (* (shift   transaction/GAS_PRICE           tx-finl---failure---row-offset---transaction-row)
                                                           (shift   transaction/REFUND_EFFECTIVE    tx-finl---failure---row-offset---transaction-row))
                                                        (shift      transaction/VALUE               tx-finl---failure---row-offset---transaction-row)))

;; recipient account
;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;
(defconstraint   tx-finalization---failure-setting-recipient-account-row                        (:guard (tx-finalization---failure-precondition))
                 (begin
                   (eq!     (shift account/ADDRESS_HI             tx-finl---failure---row-offset---recipient-account-row)     (shift    transaction/TO_ADDRESS_HI    tx-finl---failure---row-offset---transaction-row))
                   (eq!     (shift account/ADDRESS_LO             tx-finl---failure---row-offset---recipient-account-row)     (shift    transaction/TO_ADDRESS_LO    tx-finl---failure---row-offset---transaction-row))
                   (account-decrement-balance-by                  tx-finl---failure---row-offset---recipient-account-row      (shift    transaction/VALUE            tx-finl---failure---row-offset---transaction-row))
                   (account-same-nonce                            tx-finl---failure---row-offset---recipient-account-row)
                   (account-same-code                             tx-finl---failure---row-offset---recipient-account-row)
                   (account-same-deployment-number-and-status     tx-finl---failure---row-offset---recipient-account-row)
                   (account-same-warmth                           tx-finl---failure---row-offset---recipient-account-row)
                   (account-same-marked-for-selfdestruct          tx-finl---failure---row-offset---recipient-account-row)
                   (DOM-SUB-stamps---standard                     tx-finl---failure---row-offset---recipient-account-row
                                                                  1)))

;; coinbase address
;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;
(defconstraint   tx-finalization---failure-setting-coinbase-account-row                      (:guard (tx-finalization---failure-precondition))
                 (begin
                   (eq!     (shift account/ADDRESS_HI             tx-finl---failure---row-offset---coinbase-account-row)     (shift    transaction/COINBASE_ADDRESS_HI    tx-finl---failure---row-offset---transaction-row))
                   (eq!     (shift account/ADDRESS_LO             tx-finl---failure---row-offset---coinbase-account-row)     (shift    transaction/COINBASE_ADDRESS_LO    tx-finl---failure---row-offset---transaction-row))
                   (account-increment-balance-by                  tx-finl---failure---row-offset---coinbase-account-row      (tx-finalization---failure-coinbase-reward))
                   (account-same-nonce                            tx-finl---failure---row-offset---coinbase-account-row)
                   (account-same-code                             tx-finl---failure---row-offset---coinbase-account-row)
                   (account-same-deployment-number-and-status     tx-finl---failure---row-offset---coinbase-account-row)
                   (account-same-warmth                           tx-finl---failure---row-offset---coinbase-account-row)
                   (account-same-marked-for-selfdestruct          tx-finl---failure---row-offset---coinbase-account-row)
                   (DOM-SUB-stamps---standard                     tx-finl---failure---row-offset---coinbase-account-row
                                                                  2)))

;; (defun (tx-finalization---failure-coinbase-reward)
;;   (if-zero   (force-bin   (shift    transaction/IS_TYPE2          tx-finl---failure---row-offset---transaction-row))
;;              ;; TYPE 0 / TYPE 1
;;              (* (shift      transaction/GAS_PRICE                 tx-finl---failure---row-offset---transaction-row)
;;                 (- (shift   transaction/GAS_LIMIT                 tx-finl---failure---row-offset---transaction-row)
;;                    (shift   transaction/REFUND_EFFECTIVE          tx-finl---failure---row-offset---transaction-row)))
;;              ;; TYPE 2
;;              (* (- (shift   transaction/GAS_PRICE                 tx-finl---failure---row-offset---transaction-row)
;;                    (shift   transaction/BASEFEE                   tx-finl---failure---row-offset---transaction-row))
;;                 (- (shift   transaction/GAS_LIMIT                 tx-finl---failure---row-offset---transaction-row)
;;                    (shift   transaction/REFUND_EFFECTIVE          tx-finl---failure---row-offset---transaction-row)))))


(defun    (tx-finalization---failure-coinbase-reward)    (shift    (*    transaction/PRIORITY_FEE_PER_GAS    (-    transaction/GAS_LIMIT    transaction/REFUND_EFFECTIVE))
                                                                   tx-finl---failure---row-offset---transaction-row))

;; justifying TXN_DATA predictions
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint   tx-finalization---failure-justifying-txn-data-prediction            (:guard (tx-finalization---failure-precondition))
                 (begin
                   (eq!   (shift transaction/STATUS_CODE               tx-finl---failure---row-offset---transaction-row)   0)
                   (eq!   (shift transaction/REFUND_COUNTER_INFINITY   tx-finl---failure---row-offset---transaction-row)   (shift   REFUND_COUNTER   -1))
                   (eq!   (shift transaction/GAS_LEFTOVER              tx-finl---failure---row-offset---transaction-row)   (shift   GAS_NEXT         -1))))
