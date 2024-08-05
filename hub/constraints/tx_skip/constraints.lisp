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
  tx-skip---sender-account---row-offset       0
  tx-skip---recipient-account---row-offset    1
  tx-skip---coinbase-account---row-offset     2
  tx-skip---transaction-row---row-offset      3)

(defconstraint tx-skip---setting-the-peeking-flags (:guard (tx-skip---precondition))
               (eq! (+ (shift PEEK_AT_ACCOUNT        tx-skip---sender-account---row-offset)
                       (shift PEEK_AT_ACCOUNT        tx-skip---recipient-account---row-offset)
                       (shift PEEK_AT_ACCOUNT        tx-skip---coinbase-account---row-offset)
                       (shift PEEK_AT_TRANSACTION    tx-skip---transaction-row---row-offset))
                    4))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                            ;;
;;   X.3 Common constraints   ;;
;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (tx-skip---wei-cost-for-sender)
  (+ (shift    transaction/VALUE                         tx-skip---transaction-row---row-offset)
     (* (shift transaction/GAS_PRICE                     tx-skip---transaction-row---row-offset)
        (- (shift transaction/GAS_LIMIT                  tx-skip---transaction-row---row-offset)
           (shift transaction/GAS_INITIALLY_AVAILABLE    tx-skip---transaction-row---row-offset)))))

(defun (tx-skip---is-deployment)
  (force-bin (shift transaction/IS_DEPLOYMENT tx-skip---transaction-row---row-offset)))

;; sender account operation
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint tx-skip---setting-sender-account-row (:guard (tx-skip---precondition))
               (begin
                 (eq!    (shift account/ADDRESS_HI             tx-skip---sender-account---row-offset)
                         (shift transaction/FROM_ADDRESS_HI    tx-skip---transaction-row---row-offset))
                 (eq!    (shift account/ADDRESS_LO             tx-skip---sender-account---row-offset)
                         (shift transaction/FROM_ADDRESS_LO    tx-skip---transaction-row---row-offset))
                 (account-decrement-balance-by                 tx-skip---sender-account---row-offset    (tx-skip---wei-cost-for-sender))
                 (account-increment-nonce                      tx-skip---sender-account---row-offset)
                 (account-same-code                            tx-skip---sender-account---row-offset)
                 (account-same-deployment-number-and-status    tx-skip---sender-account---row-offset)
                 (account-same-warmth                          tx-skip---sender-account---row-offset)
                 (account-same-marked-for-selfdestruct         tx-skip---sender-account---row-offset)
                 (account-isnt-precompile                      tx-skip---sender-account---row-offset)
                 (DOM-SUB-stamps---standard                    tx-skip---sender-account---row-offset    0)))

;; recipient account operation
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint tx-skip---setting-recipient-account-row (:guard (tx-skip---precondition))
               (begin (eq!    (shift account/ADDRESS_HI           tx-skip---recipient-account---row-offset)
                              (shift transaction/TO_ADDRESS_HI    tx-skip---transaction-row---row-offset))
                      (eq!    (shift account/ADDRESS_LO           tx-skip---recipient-account---row-offset)
                              (shift transaction/TO_ADDRESS_LO    tx-skip---transaction-row---row-offset))
                      (account-increment-balance-by               tx-skip---recipient-account---row-offset
                                                                  (shift transaction/VALUE tx-skip---transaction-row---row-offset))
                      ;; (account-increment-nonce                       tx-skip---recipient-account---row-offset)
                      ;; (account-same-code                             tx-skip---recipient-account---row-offset)
                      ;; (account-same-deployment-number-and-status     tx-skip---recipient-account---row-offset)
                      (account-same-warmth                        tx-skip---recipient-account---row-offset)
                      (account-same-marked-for-selfdestruct       tx-skip---recipient-account---row-offset)
                      (account-isnt-precompile                    tx-skip---recipient-account---row-offset)
                      (DOM-SUB-stamps---standard                  tx-skip---recipient-account---row-offset 1)))

(defconstraint tx-skip---setting-recipient-account-row-nonce-code-and-deployment-status-for-trivial-message-calls (:guard (tx-skip---precondition))
               (if-zero (tx-skip---is-deployment)
                        ;; deployment ≡ 0 i.e. pure transfers
                        (begin    (account-same-nonce tx-skip---recipient-account---row-offset)
                                  (account-same-code tx-skip---recipient-account---row-offset)
                                  (account-same-deployment-number-and-status tx-skip---recipient-account---row-offset))))

(defconstraint tx-skip---setting-recipient-account-row-nonce-for-trivial-deployments (:guard (tx-skip---precondition))
               (if-not-zero    (tx-skip---is-deployment)
                               ;; deployment ≡ 1 i.e. trivial deployments
                               (begin  ;; nonce
                                 (account-increment-nonce tx-skip---recipient-account---row-offset)
                                 (debug (vanishes! (shift account/NONCE tx-skip---recipient-account---row-offset))))))

(defconstraint tx-skip---setting-recipient-account-row-code-for-trivial-deployments (:guard (tx-skip---precondition))
               (if-not-zero    (tx-skip---is-deployment)
                               ;; deployment ≡ 1 i.e. trivial deployments
                               (begin  ;; code
                                 ;; ;; current code
                                 (debug (eq! (shift account/HAS_CODE tx-skip---recipient-account---row-offset) 0))
                                 (debug (eq! (shift account/CODE_HASH_HI tx-skip---recipient-account---row-offset)
                                             EMPTY_KECCAK_HI))
                                 (debug (eq! (shift account/CODE_HASH_LO tx-skip---recipient-account---row-offset)
                                             EMPTY_KECCAK_LO))
                                 (debug (eq! (shift account/CODE_SIZE tx-skip---recipient-account---row-offset) 0))
                                 ;; ;; updated code
                                 (eq! (shift account/HAS_CODE_NEW tx-skip---recipient-account---row-offset) 0)
                                 (debug (eq! (shift account/CODE_HASH_HI tx-skip---recipient-account---row-offset)
                                             EMPTY_KECCAK_HI))
                                 (debug (eq! (shift account/CODE_HASH_LO tx-skip---recipient-account---row-offset)
                                             EMPTY_KECCAK_LO))
                                 (eq! (shift account/CODE_SIZE tx-skip---recipient-account---row-offset)
                                      (shift transaction/INIT_CODE_SIZE tx-skip---transaction-row---row-offset)))))

(defconstraint tx-skip---setting-recipient-account-row-deployment-status-and-number-for-trivial-deployments (:guard (tx-skip---precondition))
               (if-not-zero    (tx-skip---is-deployment)
                               ;; deployment ≡ 1 i.e. trivial deployments
                               (begin  ;; deployment
                                 (account-increment-deployment-number tx-skip---recipient-account---row-offset)
                                 (debug (eq! (shift account/DEPLOYMENT_STATUS tx-skip---recipient-account---row-offset) 0))
                                 (eq! (shift account/DEPLOYMENT_STATUS_NEW tx-skip---recipient-account---row-offset) 1))))

(defconstraint tx-skip---recipient-is-no-precompile (:guard (tx-skip---precondition))
               (if-not-zero    (tx-skip---is-deployment)
                               ;; deployment ≡ 1 i.e. trivial deployments
                               (begin (account-trim-address tx-skip---recipient-account---row-offset
                                                            (shift transaction/TO_ADDRESS_HI tx-skip---transaction-row---row-offset)
                                                            (shift transaction/TO_ADDRESS_LO tx-skip---transaction-row---row-offset))
                                      (account-isnt-precompile tx-skip---recipient-account---row-offset))))

;; coinbase account operation
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; (defun (tx-skip---coinbase-fee)
;;   (if-zero   (force-bin   (shift    transaction/IS_TYPE2          tx-skip---transaction-row---row-offset))
;;              ;; TYPE 0 / TYPE 1
;;              (*    (shift      transaction/GAS_PRICE              tx-skip---transaction-row---row-offset)
;;                    (-    (shift   transaction/GAS_LIMIT           tx-skip---transaction-row---row-offset)
;;                          (shift   transaction/REFUND_EFFECTIVE    tx-skip---transaction-row---row-offset)))
;;              ;; TYPE 2
;;              (*    (-    (shift   transaction/GAS_PRICE           tx-skip---transaction-row---row-offset)
;;                          (shift   transaction/BASEFEE             tx-skip---transaction-row---row-offset))
;;                    (-    (shift   transaction/GAS_LIMIT           tx-skip---transaction-row---row-offset)
;;                          (shift   transaction/REFUND_EFFECTIVE    tx-skip---transaction-row---row-offset)))))

(defun    (tx-skip---coinbase-fee)    (shift    (*    transaction/PRIORITY_FEE_PER_GAS    (-    transaction/GAS_LIMIT    transaction/REFUND_EFFECTIVE))
                                                tx-skip---transaction-row---row-offset))

(defconstraint tx-skip---setting-coinbase-account-row (:guard (tx-skip---precondition))
               (begin 
                 (eq!    (shift account/ADDRESS_HI tx-skip---coinbase-account---row-offset)
                         (shift transaction/COINBASE_ADDRESS_HI tx-skip---transaction-row---row-offset))
                 (eq!    (shift account/ADDRESS_LO tx-skip---coinbase-account---row-offset)
                         (shift transaction/COINBASE_ADDRESS_LO tx-skip---transaction-row---row-offset))
                 (account-increment-balance-by                 tx-skip---coinbase-account---row-offset (tx-skip---coinbase-fee))
                 (account-same-nonce                           tx-skip---coinbase-account---row-offset)
                 (account-same-code                            tx-skip---coinbase-account---row-offset)
                 (account-same-deployment-number-and-status    tx-skip---coinbase-account---row-offset)
                 (account-same-warmth                          tx-skip---coinbase-account---row-offset)
                 (account-same-marked-for-selfdestruct         tx-skip---coinbase-account---row-offset)
                 (DOM-SUB-stamps---standard                    tx-skip---coinbase-account---row-offset    2)))

(defconstraint tx-skip---transaction-row-partially-justifying-requires-evm-execution (:guard (tx-skip---precondition))
               (begin
                 (vanishes!    (shift transaction/REQUIRES_EVM_EXECUTION          tx-skip---transaction-row---row-offset))
                 (if-zero      (shift transaction/IS_DEPLOYMENT                   tx-skip---transaction-row---row-offset)
                               (vanishes!    (shift account/HAS_CODE              tx-skip---recipient-account---row-offset))
                               (vanishes!    (shift transaction/INIT_CODE_SIZE    tx-skip---transaction-row---row-offset)))))

(defconstraint tx-skip---transaction-row-justifying-total-accrued-refunds (:guard (tx-skip---precondition))
               (vanishes! (shift transaction/REFUND_COUNTER_INFINITY tx-skip---transaction-row---row-offset)))

(defconstraint tx-skip---transaction-row-justifying-initial-balance (:guard (tx-skip---precondition))
               (eq! (shift account/BALANCE tx-skip---sender-account---row-offset)
                    (shift transaction/INITIAL_BALANCE tx-skip---transaction-row---row-offset)))

(defconstraint tx-skip---transaction-row-justifying-status-code (:guard (tx-skip---precondition))
               (eq! (shift transaction/STATUS_CODE tx-skip---transaction-row---row-offset) 1))

(defconstraint tx-skip---transaction-row-justifying-nonce (:guard (tx-skip---precondition))
               (eq! (shift transaction/NONCE    tx-skip---transaction-row---row-offset)
                    (shift account/NONCE        tx-skip---sender-account---row-offset)))

(defconstraint tx-skip---transaction-row-justifying-left-over-gas (:guard (tx-skip---precondition))
               (eq! (shift transaction/GAS_LEFTOVER               tx-skip---transaction-row---row-offset)
                    (shift transaction/GAS_INITIALLY_AVAILABLE    tx-skip---transaction-row---row-offset)))

(defconstraint tx-skip---transaction-row-justifying-effective-refund (:guard (tx-skip---precondition))
               (eq! (shift transaction/REFUND_EFFECTIVE    tx-skip---transaction-row---row-offset)
                    (shift transaction/GAS_LEFTOVER        tx-skip---transaction-row---row-offset)))


