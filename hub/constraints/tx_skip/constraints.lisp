(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                   ;;
;;   X.1 Introduction                ;;
;;   X.2 Setting the peeking flags   ;;
;;                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (tx-skip-precondition) (* TX_SKIP (remained-constant HUB_STAMP)))

(defconst
  TX_SKIP_SENDER_ACCOUNT_ROW_OFFSET       0
  TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET    1
  TX_SKIP_COINBASE_ACCOUNT_ROW_OFFSET     2
  TX_SKIP_TRANSACTION_ROW_OFFSET          3
  )

(defconstraint   tx-skip-setting-the-peeking-flags (:guard (tx-skip-precondition))
                 (eq! (+ (shift PEEK_AT_ACCOUNT        TX_SKIP_SENDER_ACCOUNT_ROW_OFFSET    )
                         (shift PEEK_AT_ACCOUNT        TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET )
                         (shift PEEK_AT_ACCOUNT        TX_SKIP_COINBASE_ACCOUNT_ROW_OFFSET  )
                         (shift PEEK_AT_TRANSACTION    TX_SKIP_TRANSACTION_ROW_OFFSET       ))
                      4))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                            ;;
;;   X.3 Common constraints   ;;
;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (tx-skip-wei-cost-for-sender) (+ (shift transaction/VALUE                                TX_SKIP_TRANSACTION_ROW_OFFSET)
                                        (* (shift transaction/GAS_PRICE                         TX_SKIP_TRANSACTION_ROW_OFFSET)
                                           (- (shift transaction/GAS_LIMIT                      TX_SKIP_TRANSACTION_ROW_OFFSET)
                                              (shift transaction/GAS_INITIALLY_AVAILABLE        TX_SKIP_TRANSACTION_ROW_OFFSET)))))

(defun (tx-skip-is-deployment)       (force-bin (shift transaction/IS_DEPLOYMENT            TX_SKIP_TRANSACTION_ROW_OFFSET)))

;; sender account operation
;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   tx-skip-setting-sender-account-row                        (:guard (tx-skip-precondition))
                 (begin
                   (eq!     (shift account/ADDRESS_HI             TX_SKIP_SENDER_ACCOUNT_ROW_OFFSET)     (shift transaction/FROM_ADDRESS_HI TX_SKIP_TRANSACTION_ROW_OFFSET))
                   (eq!     (shift account/ADDRESS_LO             TX_SKIP_SENDER_ACCOUNT_ROW_OFFSET)     (shift transaction/FROM_ADDRESS_LO TX_SKIP_TRANSACTION_ROW_OFFSET))
                   (account-decrement-balance-by                  TX_SKIP_SENDER_ACCOUNT_ROW_OFFSET      (tx-skip-wei-cost-for-sender))
                   (account-increment-nonce                       TX_SKIP_SENDER_ACCOUNT_ROW_OFFSET)
                   (account-same-code                             TX_SKIP_SENDER_ACCOUNT_ROW_OFFSET)
                   (account-same-deployment-number-and-status     TX_SKIP_SENDER_ACCOUNT_ROW_OFFSET)
                   (account-same-warmth                           TX_SKIP_SENDER_ACCOUNT_ROW_OFFSET)
                   (account-same-marked-for-selfdestruct          TX_SKIP_SENDER_ACCOUNT_ROW_OFFSET)
                   (account-isnt-precompile                       TX_SKIP_SENDER_ACCOUNT_ROW_OFFSET)
                   (standard-dom-sub-stamps                       TX_SKIP_SENDER_ACCOUNT_ROW_OFFSET
                                                                  0)))

;; recipient account operation
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   tx-skip-setting-recipient-account-row                     (:guard (tx-skip-precondition))
                 (begin
                   (eq!     (shift account/ADDRESS_HI             TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET)     (shift transaction/TO_ADDRESS_HI     TX_SKIP_TRANSACTION_ROW_OFFSET))
                   (eq!     (shift account/ADDRESS_LO             TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET)     (shift transaction/TO_ADDRESS_LO     TX_SKIP_TRANSACTION_ROW_OFFSET))
                   (account-increment-balance-by                  TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET      (shift transaction/VALUE             TX_SKIP_TRANSACTION_ROW_OFFSET))
                   ;; (account-increment-nonce                       TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET)
                   ;; (account-same-code                             TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET)
                   ;; (account-same-deployment-number-and-status     TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET)
                   (account-same-warmth                           TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET)
                   (account-same-marked-for-selfdestruct          TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET)
                   (account-isnt-precompile                       TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET)
                   (standard-dom-sub-stamps                       TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET
                                                                  1)))

(defconstraint   tx-skip-setting-recipient-account-row-nonce-code-and-deployment-status-for-trivial-message-calls     (:guard (tx-skip-precondition))
                 (if-zero (tx-skip-is-deployment)
                          ;; deployment ≡ 0 i.e. pure transfers
                          (begin
                            (account-same-nonce                                        TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET)
                            (account-same-code                                         TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET)
                            (account-same-deployment-number-and-status                 TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET))))

(defconstraint   tx-skip-setting-recipient-account-row-nonce-for-trivial-deployments     (:guard (tx-skip-precondition))
                 (if-not-zero (tx-skip-is-deployment)
                              ;; deployment ≡ 1 i.e. trivial deployments
                              (begin
                                ;; nonce
                                (account-increment-nonce                                   TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET)
                                (debug (vanishes! (shift account/NONCE                     TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET))))))

(defconstraint   tx-skip-setting-recipient-account-row-code-for-trivial-deployments     (:guard (tx-skip-precondition))
                 (if-not-zero (tx-skip-is-deployment)
                              ;; deployment ≡ 1 i.e. trivial deployments
                              (begin
                                ;; code
                                ;; ;; current code
                                (debug (eq! (shift account/HAS_CODE                        TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET) 0))
                                (debug (eq! (shift account/CODE_HASH_HI                    TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET) EMPTY_KECCAK_HI))
                                (debug (eq! (shift account/CODE_HASH_LO                    TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET) EMPTY_KECCAK_LO))
                                (debug (eq! (shift account/CODE_SIZE                       TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET) 0))
                                ;; ;; updated code
                                (eq! (shift account/HAS_CODE_NEW                           TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET) 0)
                                (debug (eq! (shift account/CODE_HASH_HI                    TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET) EMPTY_KECCAK_HI))
                                (debug (eq! (shift account/CODE_HASH_LO                    TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET) EMPTY_KECCAK_LO))
                                (eq! (shift account/CODE_SIZE                              TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET)
                                     (shift transaction/INIT_CODE_SIZE                     TX_SKIP_TRANSACTION_ROW_OFFSET)))))

(defconstraint   tx-skip-setting-recipient-account-row-deployment-status-and-number-for-trivial-deployments     (:guard (tx-skip-precondition))
                 (if-not-zero (tx-skip-is-deployment)
                              ;; deployment ≡ 1 i.e. trivial deployments
                              (begin
                                ;; deployment
                                (account-increment-deployment-number                       TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET)
                                (debug (eq! (shift account/DEPLOYMENT_STATUS               TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET) 0))
                                (eq!        (shift account/DEPLOYMENT_STATUS_NEW           TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET) 1))))

(defconstraint   tx-skip-recipient-is-no-precompile                  (:guard (tx-skip-precondition))
                 (if-not-zero (tx-skip-is-deployment)
                              ;; deployment ≡ 1 i.e. trivial deployments
                              (begin
                                (account-trim-address            TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET
                                                                 (shift    transaction/TO_ADDRESS_HI    TX_SKIP_TRANSACTION_ROW_OFFSET)
                                                                 (shift    transaction/TO_ADDRESS_LO    TX_SKIP_TRANSACTION_ROW_OFFSET))
                                (account-isnt-precompile         TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET))))


;; coinbase account operation
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (coinbase-fee) (* (shift     transaction/PRIORITY_FEE_PER_GAS   TX_SKIP_TRANSACTION_ROW_OFFSET)
                         (- (shift  transaction/GAS_LIMIT              TX_SKIP_TRANSACTION_ROW_OFFSET)
                            (shift  transaction/REFUND_EFFECTIVE       TX_SKIP_TRANSACTION_ROW_OFFSET))))

(defconstraint   tx-skip-setting-coinbase-account-row                      (:guard (tx-skip-precondition))
                 (begin
                   (eq!     (shift account/ADDRESS_HI             TX_SKIP_COINBASE_ACCOUNT_ROW_OFFSET)     (shift transaction/COINBASE_ADDRESS_HI TX_SKIP_TRANSACTION_ROW_OFFSET))
                   (eq!     (shift account/ADDRESS_LO             TX_SKIP_COINBASE_ACCOUNT_ROW_OFFSET)     (shift transaction/COINBASE_ADDRESS_LO TX_SKIP_TRANSACTION_ROW_OFFSET))
                   (account-increment-balance-by                  TX_SKIP_COINBASE_ACCOUNT_ROW_OFFSET      (coinbase-fee))
                   (account-same-nonce                            TX_SKIP_COINBASE_ACCOUNT_ROW_OFFSET)
                   (account-same-code                             TX_SKIP_COINBASE_ACCOUNT_ROW_OFFSET)
                   (account-same-deployment-number-and-status     TX_SKIP_COINBASE_ACCOUNT_ROW_OFFSET)
                   (account-same-warmth                           TX_SKIP_COINBASE_ACCOUNT_ROW_OFFSET)
                   (account-same-marked-for-selfdestruct          TX_SKIP_COINBASE_ACCOUNT_ROW_OFFSET)
                   (standard-dom-sub-stamps                       TX_SKIP_COINBASE_ACCOUNT_ROW_OFFSET
                                                                  2)))

(defconstraint   tx-skip-transaction-row-partially-justifying-requires-evm-execution           (:guard (tx-skip-precondition))
                 (begin
                   (vanishes! (shift     transaction/REQUIRES_EVM_EXECUTION       TX_SKIP_TRANSACTION_ROW_OFFSET))
                   (if-zero   (shift     transaction/IS_DEPLOYMENT                TX_SKIP_TRANSACTION_ROW_OFFSET)
                              (vanishes! (shift account/HAS_CODE                  TX_SKIP_RECIPIENT_ACCOUNT_ROW_OFFSET))
                              (vanishes! (shift transaction/INIT_CODE_SIZE        TX_SKIP_TRANSACTION_ROW_OFFSET)))))


(defconstraint   tx-skip-transaction-row-justifying-total-accrued-refunds            (:guard (tx-skip-precondition))
                 (vanishes! (shift   transaction/REFUND_COUNTER_INFINITY          TX_SKIP_TRANSACTION_ROW_OFFSET)))

(defconstraint   tx-skip-transaction-row-justifying-initial-balance                  (:guard (tx-skip-precondition))
                 (eq!   (shift   account/BALANCE                     TX_SKIP_SENDER_ACCOUNT_ROW_OFFSET)
                        (shift   transaction/INITIAL_BALANCE         TX_SKIP_TRANSACTION_ROW_OFFSET)))

(defconstraint   tx-skip-transaction-row-justifying-status-code                      (:guard (tx-skip-precondition))
                 (eq!   (shift   transaction/STATUS_CODE             TX_SKIP_TRANSACTION_ROW_OFFSET)
                        1))

(defconstraint   tx-skip-transaction-row-justifying-nonce                            (:guard (tx-skip-precondition))
                 (eq!   (shift   transaction/NONCE                   TX_SKIP_TRANSACTION_ROW_OFFSET)
                        (shift   account/NONCE                       TX_SKIP_SENDER_ACCOUNT_ROW_OFFSET)))

(defconstraint   tx-skip-transaction-row-justifying-left-over-gas                    (:guard (tx-skip-precondition))
                 (eq!   (shift   transaction/GAS_LEFTOVER            TX_SKIP_TRANSACTION_ROW_OFFSET)
                        (shift   transaction/GAS_INITIALLY_AVAILABLE             TX_SKIP_TRANSACTION_ROW_OFFSET)))

(defconstraint   tx-skip-transaction-row-justifying-effective-refund                 (:guard (tx-skip-precondition))
                 (eq!   (shift   transaction/REFUND_EFFECTIVE        TX_SKIP_TRANSACTION_ROW_OFFSET)
                        (shift   transaction/GAS_LEFTOVER            TX_SKIP_TRANSACTION_ROW_OFFSET)))
