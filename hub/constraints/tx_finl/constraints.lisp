(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;   X Finalization phase   ;;
;;   X.1 Introduction       ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconst
  ;; transaction success
  ROW_OFFSET_TX_FINL_SUCCESS_SENDER_ACCOUNT      0
  ROW_OFFSET_TX_FINL_SUCCESS_COINBASE_ACCOUNT    1
  ROW_OFFSET_TX_FINL_SUCCESS_TRANSACTION_ROW     2
  NUMBER_OF_ROWS_TX_FINL_SUCCESS                 3
  ;; transaction failure
  ROW_OFFSET_TX_FINL_FAILURE_SENDER_ACCOUNT      0
  ROW_OFFSET_TX_FINL_FAILURE_RECIPIENT_ACCOUNT   1
  ROW_OFFSET_TX_FINL_FAILURE_COINBASE_ACCOUNT    2
  ROW_OFFSET_TX_FINL_FAILURE_TRANSACTION_ROW     3
  NUMBER_OF_ROWS_TX_FINL_FAILURE                 4
)


;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;   X.2 Peeking flags   ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (tx-finalization-precondition)         (* (prev TX_EXEC) TX_FINL))

(defconstraint   tx-finalization-setting-peeking-flags (:guard (tx-finalization-precondition))
                 (if-zero   (prev CONTEXT_WILL_REVERT)
                            ;; cn_will_rev ≡ 0
                            (eq! NUMBER_OF_ROWS_TX_FINL_SUCCESS
                                 (+   (shift PEEK_AT_ACCOUNT        ROW_OFFSET_TX_FINL_SUCCESS_SENDER_ACCOUNT    )
                                      (shift PEEK_AT_ACCOUNT        ROW_OFFSET_TX_FINL_SUCCESS_COINBASE_ACCOUNT  )
                                      (shift PEEK_AT_TRANSACTION    ROW_OFFSET_TX_FINL_SUCCESS_TRANSACTION_ROW       )))
                            ;; cn_will_rev ≡ 1
                            (eq! NUMBER_OF_ROWS_TX_FINL_FAILURE
                                 (+   (shift PEEK_AT_ACCOUNT        ROW_OFFSET_TX_FINL_FAILURE_SENDER_ACCOUNT    )
                                      (shift PEEK_AT_ACCOUNT        ROW_OFFSET_TX_FINL_FAILURE_RECIPIENT_ACCOUNT )
                                      (shift PEEK_AT_ACCOUNT        ROW_OFFSET_TX_FINL_FAILURE_COINBASE_ACCOUNT  )
                                      (shift PEEK_AT_TRANSACTION    ROW_OFFSET_TX_FINL_FAILURE_TRANSACTION_ROW       )))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;   X.3 Transaction SUCCESS   ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (tx-finalization-success-precondition) (* (tx-finalization-precondition) (prev (- 1 CONTEXT_WILL_REVERT))))

;; sender account
;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;
(defconstraint   tx-finalization-success-setting-sender-account-row                        (:guard (tx-finalization-success-precondition))
                 (begin
                   (eq!     (shift account/ADDRESS_HI             ROW_OFFSET_TX_FINL_SUCCESS_SENDER_ACCOUNT)     (shift    transaction/FROM_ADDRESS_HI    ROW_OFFSET_TX_FINL_SUCCESS_TRANSACTION_ROW))
                   (eq!     (shift account/ADDRESS_LO             ROW_OFFSET_TX_FINL_SUCCESS_SENDER_ACCOUNT)     (shift    transaction/FROM_ADDRESS_LO    ROW_OFFSET_TX_FINL_SUCCESS_TRANSACTION_ROW))
                   (account-increment-balance-by                  ROW_OFFSET_TX_FINL_SUCCESS_SENDER_ACCOUNT      (tx-finalization-success-sender-refund))
                   (account-same-nonce                            ROW_OFFSET_TX_FINL_SUCCESS_SENDER_ACCOUNT)
                   (account-same-code                             ROW_OFFSET_TX_FINL_SUCCESS_SENDER_ACCOUNT)
                   (account-same-deployment-number-and-status     ROW_OFFSET_TX_FINL_SUCCESS_SENDER_ACCOUNT)
                   (account-same-warmth                           ROW_OFFSET_TX_FINL_SUCCESS_SENDER_ACCOUNT)
                   (account-same-marked-for-selfdestruct          ROW_OFFSET_TX_FINL_SUCCESS_SENDER_ACCOUNT)
                   (standard-dom-sub-stamps                       ROW_OFFSET_TX_FINL_SUCCESS_SENDER_ACCOUNT
                                                                  0)))

(defun (tx-finalization-success-sender-refund)    (* (shift   transaction/GAS_PRICE           ROW_OFFSET_TX_FINL_SUCCESS_TRANSACTION_ROW)
                                                     (shift   transaction/REFUND_EFFECTIVE    ROW_OFFSET_TX_FINL_SUCCESS_TRANSACTION_ROW)))

;; coinbase address
;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;
(defconstraint   tx-finalization-success-setting-coinbase-account-row                      (:guard (tx-finalization-success-precondition))
                 (begin
                   (eq!     (shift account/ADDRESS_HI             ROW_OFFSET_TX_FINL_SUCCESS_COINBASE_ACCOUNT)     (shift    transaction/COINBASE_ADDRESS_HI    ROW_OFFSET_TX_FINL_SUCCESS_TRANSACTION_ROW))
                   (eq!     (shift account/ADDRESS_LO             ROW_OFFSET_TX_FINL_SUCCESS_COINBASE_ACCOUNT)     (shift    transaction/COINBASE_ADDRESS_LO    ROW_OFFSET_TX_FINL_SUCCESS_TRANSACTION_ROW))
                   (account-increment-balance-by                  ROW_OFFSET_TX_FINL_SUCCESS_COINBASE_ACCOUNT      (tx-finalization-success-coinbase-reward))
                   (account-same-nonce                            ROW_OFFSET_TX_FINL_SUCCESS_COINBASE_ACCOUNT)
                   (account-same-code                             ROW_OFFSET_TX_FINL_SUCCESS_COINBASE_ACCOUNT)
                   (account-same-deployment-number-and-status     ROW_OFFSET_TX_FINL_SUCCESS_COINBASE_ACCOUNT)
                   (account-same-warmth                           ROW_OFFSET_TX_FINL_SUCCESS_COINBASE_ACCOUNT)
                   (account-same-marked-for-selfdestruct          ROW_OFFSET_TX_FINL_SUCCESS_COINBASE_ACCOUNT)
                   (standard-dom-sub-stamps                       ROW_OFFSET_TX_FINL_SUCCESS_COINBASE_ACCOUNT
                                                                  1)))

(defun (tx-finalization-success-coinbase-reward)
  (if-zero   (force-bin   (shift    transaction/IS_TYPE2          ROW_OFFSET_TX_FINL_SUCCESS_TRANSACTION_ROW))
             ;; TYPE 0 / TYPE 1
             (* (shift      transaction/GAS_PRICE                 ROW_OFFSET_TX_FINL_SUCCESS_TRANSACTION_ROW)
                (- (shift   transaction/GAS_LIMIT                 ROW_OFFSET_TX_FINL_SUCCESS_TRANSACTION_ROW)
                   (shift   transaction/REFUND_EFFECTIVE          ROW_OFFSET_TX_FINL_SUCCESS_TRANSACTION_ROW)))
             ;; TYPE 2
             (* (- (shift   transaction/GAS_PRICE                 ROW_OFFSET_TX_FINL_SUCCESS_TRANSACTION_ROW)
                   (shift   transaction/BASEFEE                   ROW_OFFSET_TX_FINL_SUCCESS_TRANSACTION_ROW))
                (- (shift   transaction/GAS_LIMIT                 ROW_OFFSET_TX_FINL_SUCCESS_TRANSACTION_ROW)
                   (shift   transaction/REFUND_EFFECTIVE          ROW_OFFSET_TX_FINL_SUCCESS_TRANSACTION_ROW)))))

;; justifying TXN_DATA predictions
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint   tx-finalization-success-justifying-txn-data-prediction            (:guard (tx-finalization-success-precondition))
                 (begin
                   (eq!   (shift transaction/STATUS_CODE               ROW_OFFSET_TX_FINL_SUCCESS_TRANSACTION_ROW)   1)
                   (eq!   (shift transaction/REFUND_COUNTER_INFINITY   ROW_OFFSET_TX_FINL_SUCCESS_TRANSACTION_ROW)   (shift   REFUND_COUNTER   -1))
                   (eq!   (shift transaction/GAS_LEFTOVER              ROW_OFFSET_TX_FINL_SUCCESS_TRANSACTION_ROW)   (shift   GAS_NEXT         -1))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;   X.3 Transaction FAILURE   ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (tx-finalization-failure-precondition) (* (tx-finalization-precondition) (prev CONTEXT_WILL_REVERT)))

;; sender account
;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;
(defconstraint   tx-finalization-failure-setting-sender-account-row                        (:guard (tx-finalization-failure-precondition))
                 (begin
                   (eq!     (shift account/ADDRESS_HI             ROW_OFFSET_TX_FINL_FAILURE_SENDER_ACCOUNT)     (shift    transaction/FROM_ADDRESS_HI    ROW_OFFSET_TX_FINL_FAILURE_TRANSACTION_ROW))
                   (eq!     (shift account/ADDRESS_LO             ROW_OFFSET_TX_FINL_FAILURE_SENDER_ACCOUNT)     (shift    transaction/FROM_ADDRESS_LO    ROW_OFFSET_TX_FINL_FAILURE_TRANSACTION_ROW))
                   (account-increment-balance-by                  ROW_OFFSET_TX_FINL_FAILURE_SENDER_ACCOUNT      (tx-finalization-failure-sender-refund))
                   (account-same-nonce                            ROW_OFFSET_TX_FINL_FAILURE_SENDER_ACCOUNT)
                   (account-same-code                             ROW_OFFSET_TX_FINL_FAILURE_SENDER_ACCOUNT)
                   (account-same-deployment-number-and-status     ROW_OFFSET_TX_FINL_FAILURE_SENDER_ACCOUNT)
                   (account-same-warmth                           ROW_OFFSET_TX_FINL_FAILURE_SENDER_ACCOUNT)
                   (account-same-marked-for-selfdestruct          ROW_OFFSET_TX_FINL_FAILURE_SENDER_ACCOUNT)
                   (standard-dom-sub-stamps                       ROW_OFFSET_TX_FINL_FAILURE_SENDER_ACCOUNT
                                                                  0)))

(defun (tx-finalization-failure-sender-refund)    (+  (* (shift   transaction/GAS_PRICE           ROW_OFFSET_TX_FINL_FAILURE_TRANSACTION_ROW)
                                                         (shift   transaction/REFUND_EFFECTIVE    ROW_OFFSET_TX_FINL_FAILURE_TRANSACTION_ROW))
                                                      (shift      transaction/VALUE               ROW_OFFSET_TX_FINL_FAILURE_TRANSACTION_ROW)))

;; recipient account
;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;
(defconstraint   tx-finalization-failure-setting-recipient-account-row                        (:guard (tx-finalization-failure-precondition))
                 (begin
                   (eq!     (shift account/ADDRESS_HI             ROW_OFFSET_TX_FINL_FAILURE_RECIPIENT_ACCOUNT)     (shift    transaction/TO_ADDRESS_HI    ROW_OFFSET_TX_FINL_FAILURE_TRANSACTION_ROW))
                   (eq!     (shift account/ADDRESS_LO             ROW_OFFSET_TX_FINL_FAILURE_RECIPIENT_ACCOUNT)     (shift    transaction/TO_ADDRESS_LO    ROW_OFFSET_TX_FINL_FAILURE_TRANSACTION_ROW))
                   (account-decrement-balance-by                  ROW_OFFSET_TX_FINL_FAILURE_RECIPIENT_ACCOUNT      (shift    transaction/VALUE            ROW_OFFSET_TX_FINL_FAILURE_TRANSACTION_ROW))
                   (account-same-nonce                            ROW_OFFSET_TX_FINL_FAILURE_RECIPIENT_ACCOUNT)
                   (account-same-code                             ROW_OFFSET_TX_FINL_FAILURE_RECIPIENT_ACCOUNT)
                   (account-same-deployment-number-and-status     ROW_OFFSET_TX_FINL_FAILURE_RECIPIENT_ACCOUNT)
                   (account-same-warmth                           ROW_OFFSET_TX_FINL_FAILURE_RECIPIENT_ACCOUNT)
                   (account-same-marked-for-selfdestruct          ROW_OFFSET_TX_FINL_FAILURE_RECIPIENT_ACCOUNT)
                   (standard-dom-sub-stamps                       ROW_OFFSET_TX_FINL_FAILURE_RECIPIENT_ACCOUNT
                                                                  1)))

;; coinbase address
;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;
(defconstraint   tx-finalization-failure-setting-coinbase-account-row                      (:guard (tx-finalization-failure-precondition))
                 (begin
                   (eq!     (shift account/ADDRESS_HI             ROW_OFFSET_TX_FINL_FAILURE_COINBASE_ACCOUNT)     (shift    transaction/COINBASE_ADDRESS_HI    ROW_OFFSET_TX_FINL_FAILURE_TRANSACTION_ROW))
                   (eq!     (shift account/ADDRESS_LO             ROW_OFFSET_TX_FINL_FAILURE_COINBASE_ACCOUNT)     (shift    transaction/COINBASE_ADDRESS_LO    ROW_OFFSET_TX_FINL_FAILURE_TRANSACTION_ROW))
                   (account-increment-balance-by                  ROW_OFFSET_TX_FINL_FAILURE_COINBASE_ACCOUNT      (tx-finalization-failure-coinbase-reward))
                   (account-same-nonce                            ROW_OFFSET_TX_FINL_FAILURE_COINBASE_ACCOUNT)
                   (account-same-code                             ROW_OFFSET_TX_FINL_FAILURE_COINBASE_ACCOUNT)
                   (account-same-deployment-number-and-status     ROW_OFFSET_TX_FINL_FAILURE_COINBASE_ACCOUNT)
                   (account-same-warmth                           ROW_OFFSET_TX_FINL_FAILURE_COINBASE_ACCOUNT)
                   (account-same-marked-for-selfdestruct          ROW_OFFSET_TX_FINL_FAILURE_COINBASE_ACCOUNT)
                   (standard-dom-sub-stamps                       ROW_OFFSET_TX_FINL_FAILURE_COINBASE_ACCOUNT
                                                                  2)))

(defun (tx-finalization-failure-coinbase-reward)
  (if-zero   (force-bin   (shift    transaction/IS_TYPE2          ROW_OFFSET_TX_FINL_FAILURE_TRANSACTION_ROW))
             ;; TYPE 0 / TYPE 1
             (* (shift      transaction/GAS_PRICE                 ROW_OFFSET_TX_FINL_FAILURE_TRANSACTION_ROW)
                (- (shift   transaction/GAS_LIMIT                 ROW_OFFSET_TX_FINL_FAILURE_TRANSACTION_ROW)
                   (shift   transaction/REFUND_EFFECTIVE          ROW_OFFSET_TX_FINL_FAILURE_TRANSACTION_ROW)))
             ;; TYPE 2
             (* (- (shift   transaction/GAS_PRICE                 ROW_OFFSET_TX_FINL_FAILURE_TRANSACTION_ROW)
                   (shift   transaction/BASEFEE                   ROW_OFFSET_TX_FINL_FAILURE_TRANSACTION_ROW))
                (- (shift   transaction/GAS_LIMIT                 ROW_OFFSET_TX_FINL_FAILURE_TRANSACTION_ROW)
                   (shift   transaction/REFUND_EFFECTIVE          ROW_OFFSET_TX_FINL_FAILURE_TRANSACTION_ROW)))))

;; justifying TXN_DATA predictions
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint   tx-finalization-failure-justifying-txn-data-prediction            (:guard (tx-finalization-failure-precondition))
                 (begin
                   (eq!   (shift transaction/STATUS_CODE               ROW_OFFSET_TX_FINL_FAILURE_TRANSACTION_ROW)   0)
                   (eq!   (shift transaction/REFUND_COUNTER_INFINITY   ROW_OFFSET_TX_FINL_FAILURE_TRANSACTION_ROW)   (shift   REFUND_COUNTER   -1))
                   (eq!   (shift transaction/GAS_LEFTOVER              ROW_OFFSET_TX_FINL_FAILURE_TRANSACTION_ROW)   (shift   GAS_NEXT         -1))))
