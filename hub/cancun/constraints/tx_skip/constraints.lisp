(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                   ;;
;;   X.1 Introduction                ;;
;;   X.2 Setting the peeking flags   ;;
;;                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (tx-skip---precondition)
  (* TX_SKIP
     (- HUB_STAMP (prev HUB_STAMP))))

(defconst
  tx-skip---row-offset---sender-account       0
  tx-skip---row-offset---recipient-account    1
  tx-skip---row-offset---coinbase-account     2
  tx-skip---row-offset---transaction-row      3)

(defconstraint   tx-skip---setting-the-peeking-flags (:guard (tx-skip---precondition))
                 (eq!    4
                         (+ (shift PEEK_AT_ACCOUNT        tx-skip---row-offset---sender-account)
                            (shift PEEK_AT_ACCOUNT        tx-skip---row-offset---recipient-account)
                            (shift PEEK_AT_ACCOUNT        tx-skip---row-offset---coinbase-account)
                            (shift PEEK_AT_TRANSACTION    tx-skip---row-offset---transaction-row))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                            ;;
;;   X.3 Common constraints   ;;
;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (tx-skip---wei-cost-for-sender)
  (+ (shift    transaction/VALUE                         tx-skip---row-offset---transaction-row)
     (* (shift transaction/GAS_PRICE                     tx-skip---row-offset---transaction-row)
        (- (shift transaction/GAS_LIMIT                  tx-skip---row-offset---transaction-row)
           (shift transaction/GAS_INITIALLY_AVAILABLE    tx-skip---row-offset---transaction-row)))))

(defun (tx-skip---is-deployment)
  (force-bin (shift transaction/IS_DEPLOYMENT tx-skip---row-offset---transaction-row)))

;; sender account operation
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint   tx-skip---setting-sender-account-row (:guard (tx-skip---precondition))
                 (begin
                   (eq!    (shift account/ADDRESS_HI             tx-skip---row-offset---sender-account)
                           (shift transaction/FROM_ADDRESS_HI    tx-skip---row-offset---transaction-row))
                   (eq!    (shift account/ADDRESS_LO             tx-skip---row-offset---sender-account)
                           (shift transaction/FROM_ADDRESS_LO    tx-skip---row-offset---transaction-row))
                   (account-decrement-balance-by                 tx-skip---row-offset---sender-account    (tx-skip---wei-cost-for-sender))
                   (account-increment-nonce                      tx-skip---row-offset---sender-account)
                   (account-same-code                            tx-skip---row-offset---sender-account)
                   (account-same-deployment-number-and-status    tx-skip---row-offset---sender-account)
                   (account-same-warmth                          tx-skip---row-offset---sender-account)
                   (account-same-marked-for-selfdestruct         tx-skip---row-offset---sender-account)
                   (account-isnt-precompile                      tx-skip---row-offset---sender-account)
                   (DOM-SUB-stamps---standard                    tx-skip---row-offset---sender-account    0)))


(defconstraint     tx-skip---EIP-3607---reject-transactions-from-senders-with-deployed-code          (:guard (tx-skip---precondition))
                   (vanishes!    (shift    account/HAS_CODE    tx-skip---row-offset---sender-account)))

;; recipient account operation
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint   tx-skip---setting-recipient-account-row (:guard (tx-skip---precondition))
                 (begin (eq!    (shift account/ADDRESS_HI           tx-skip---row-offset---recipient-account)
                                (shift transaction/TO_ADDRESS_HI    tx-skip---row-offset---transaction-row))
                        (eq!    (shift account/ADDRESS_LO           tx-skip---row-offset---recipient-account)
                                (shift transaction/TO_ADDRESS_LO    tx-skip---row-offset---transaction-row))
                        (account-increment-balance-by               tx-skip---row-offset---recipient-account
                                                                    (shift transaction/VALUE tx-skip---row-offset---transaction-row))
                        ;; (account-increment-nonce                       tx-skip---row-offset---recipient-account)
                        ;; (account-same-code                             tx-skip---row-offset---recipient-account)
                        ;; (account-same-deployment-number-and-status     tx-skip---row-offset---recipient-account)
                        (account-same-warmth                        tx-skip---row-offset---recipient-account)
                        (account-same-marked-for-selfdestruct       tx-skip---row-offset---recipient-account)
                        (account-isnt-precompile                    tx-skip---row-offset---recipient-account)
                        (DOM-SUB-stamps---standard                  tx-skip---row-offset---recipient-account 1)))

(defconstraint   tx-skip---recipient-account-row---trivial-message-calls---nonce-code-and-deployment-status- (:guard (tx-skip---precondition))
                 (if-zero (tx-skip---is-deployment)
                          ;; deployment ≡ 0 i.e. pure transfers
                          (begin    (account-same-nonce                          tx-skip---row-offset---recipient-account)
                                    (account-same-code                           tx-skip---row-offset---recipient-account)
                                    (account-same-deployment-number-and-status   tx-skip---row-offset---recipient-account))))

(defconstraint   tx-skip---recipient-account-row---trivial-deployments---nonce (:guard (tx-skip---precondition))
                 (if-not-zero    (tx-skip---is-deployment)
                                 ;; deployment ≡ 1 i.e. trivial deployments
                                 (begin  ;; nonce
                                   (account-increment-nonce           tx-skip---row-offset---recipient-account)
                                   (vanishes! (shift account/NONCE    tx-skip---row-offset---recipient-account)))))

(defconstraint   tx-skip---recipient-account-row---trivial-deployments---code (:guard (tx-skip---precondition))
                 (if-not-zero    (tx-skip---is-deployment)
                                 ;; deployment ≡ 1 i.e. trivial deployments
                                 (begin  ;; code
                                   ;; current code
                                   (vanishes!         (shift account/HAS_CODE        tx-skip---row-offset---recipient-account))
                                   (debug    (eq!     (shift account/CODE_HASH_HI    tx-skip---row-offset---recipient-account)    EMPTY_KECCAK_HI))
                                   (debug    (eq!     (shift account/CODE_HASH_LO    tx-skip---row-offset---recipient-account)    EMPTY_KECCAK_LO))
                                   (vanishes!         (shift account/CODE_SIZE       tx-skip---row-offset---recipient-account))
                                   ;; updated code
                                   (vanishes!         (shift account/HAS_CODE_NEW       tx-skip---row-offset---recipient-account))
                                   (debug    (eq!     (shift account/CODE_HASH_HI_NEW   tx-skip---row-offset---recipient-account)    EMPTY_KECCAK_HI))
                                   (debug    (eq!     (shift account/CODE_HASH_LO_NEW   tx-skip---row-offset---recipient-account)    EMPTY_KECCAK_LO))
                                   (eq!               (shift account/CODE_SIZE_NEW      tx-skip---row-offset---recipient-account)
                                                      (shift transaction/INIT_CODE_SIZE tx-skip---row-offset---transaction-row))
                                   (debug (vanishes!  (shift account/CODE_SIZE_NEW      tx-skip---row-offset---recipient-account))))))

(defconstraint   tx-skip---recipient-account-row---trivial-deployments---deployment-status-and-number (:guard (tx-skip---precondition))
                 (if-not-zero    (tx-skip---is-deployment)
                                 ;; deployment ≡ 1 i.e. trivial deployments
                                 (begin  ;; deployment
                                   (account-increment-deployment-number                tx-skip---row-offset---recipient-account)
                                   (debug (eq! (shift account/DEPLOYMENT_STATUS        tx-skip---row-offset---recipient-account) 0))
                                   (eq!        (shift account/DEPLOYMENT_STATUS_NEW    tx-skip---row-offset---recipient-account) 0))))

(defconstraint   tx-skip---recipient-is-no-precompile (:guard (tx-skip---precondition))
                 (if-zero    (tx-skip---is-deployment)
                             ;; deployment ≡ 0 i.e. pure transfer
                             (account-trim-address       tx-skip---row-offset---recipient-account
                                                         (shift transaction/TO_ADDRESS_HI tx-skip---row-offset---transaction-row)
                                                         (shift transaction/TO_ADDRESS_LO tx-skip---row-offset---transaction-row))))

(defun    (tx-skip---coinbase-fee)    (shift    (*    transaction/PRIORITY_FEE_PER_GAS    (-    transaction/GAS_LIMIT    transaction/REFUND_EFFECTIVE))
                                                tx-skip---row-offset---transaction-row))

(defconstraint   tx-skip---setting-coinbase-account-row (:guard (tx-skip---precondition))
                 (begin
                   (eq!    (shift account/ADDRESS_HI tx-skip---row-offset---coinbase-account)
                           (shift transaction/COINBASE_ADDRESS_HI tx-skip---row-offset---transaction-row))
                   (eq!    (shift account/ADDRESS_LO tx-skip---row-offset---coinbase-account)
                           (shift transaction/COINBASE_ADDRESS_LO tx-skip---row-offset---transaction-row))
                   (account-increment-balance-by                 tx-skip---row-offset---coinbase-account (tx-skip---coinbase-fee))
                   (account-same-nonce                           tx-skip---row-offset---coinbase-account)
                   (account-same-code                            tx-skip---row-offset---coinbase-account)
                   (account-same-deployment-number-and-status    tx-skip---row-offset---coinbase-account)
                   (account-same-warmth                          tx-skip---row-offset---coinbase-account)
                   (account-same-marked-for-selfdestruct         tx-skip---row-offset---coinbase-account)
                   (DOM-SUB-stamps---standard                    tx-skip---row-offset---coinbase-account    2)))

(defconstraint   tx-skip---transaction-row-partially-justifying-requires-evm-execution (:guard (tx-skip---precondition))
                 (begin
                   (vanishes!    (shift transaction/REQUIRES_EVM_EXECUTION          tx-skip---row-offset---transaction-row))
                   (if-zero      (shift transaction/IS_DEPLOYMENT                   tx-skip---row-offset---transaction-row)
                                 (vanishes!    (shift account/HAS_CODE              tx-skip---row-offset---recipient-account))
                                 (vanishes!    (shift transaction/INIT_CODE_SIZE    tx-skip---row-offset---transaction-row)))))

(defconstraint   tx-skip---transaction-row-justifying-total-accrued-refunds (:guard (tx-skip---precondition))
                 (vanishes! (shift transaction/REFUND_COUNTER_INFINITY tx-skip---row-offset---transaction-row)))

(defconstraint   tx-skip---transaction-row-justifying-initial-balance (:guard (tx-skip---precondition))
                 (eq! (shift account/BALANCE tx-skip---row-offset---sender-account)
                      (shift transaction/INITIAL_BALANCE tx-skip---row-offset---transaction-row)))

(defconstraint   tx-skip---transaction-row-justifying-status-code (:guard (tx-skip---precondition))
                 (eq! (shift transaction/STATUS_CODE tx-skip---row-offset---transaction-row) 1))

(defconstraint   tx-skip---transaction-row-justifying-nonce (:guard (tx-skip---precondition))
                 (eq! (shift transaction/NONCE    tx-skip---row-offset---transaction-row)
                      (shift account/NONCE        tx-skip---row-offset---sender-account)))

(defconstraint   tx-skip---transaction-row-justifying-left-over-gas (:guard (tx-skip---precondition))
                 (eq! (shift transaction/GAS_LEFTOVER               tx-skip---row-offset---transaction-row)
                      (shift transaction/GAS_INITIALLY_AVAILABLE    tx-skip---row-offset---transaction-row)))

(defconstraint   tx-skip---transaction-row-justifying-effective-refund (:guard (tx-skip---precondition))
                 (eq! (shift transaction/REFUND_EFFECTIVE    tx-skip---row-offset---transaction-row)
                      (shift transaction/GAS_LEFTOVER        tx-skip---row-offset---transaction-row)))


