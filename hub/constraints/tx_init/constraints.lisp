(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                   ;;
;;   X.1 Introduction                ;;
;;   X.2 Setting the peeking flags   ;;
;;                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; TX_INIT[i - 1] = 0  && TX_INIT[i] = 1
(defun (tx-init---standard-precondition) (* (- 1 (shift TX_INIT -1))
                                 TX_INIT))

(defconst
  TX_INIT_SENDER_ACCOUNT_ROW_OFFSET         0
  TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET      1
  TX_INIT_MISCELLANEOUS_ROW_OFFSET          2
  TX_INIT_CONTEXT_INITIALIZATION_ROW_OFFSET 3
  TX_INIT_TRANSACTION_ROW_OFFSET            4
  )

(defconstraint   tx-initialization---setting-the-peeking-flags (:guard (tx-init---standard-precondition))
                 (eq! (+ (shift PEEK_AT_ACCOUNT             TX_INIT_SENDER_ACCOUNT_ROW_OFFSET         )
                         (shift PEEK_AT_ACCOUNT             TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET      )
                         (shift PEEK_AT_MISCELLANEOUS       TX_INIT_MISCELLANEOUS_ROW_OFFSET          )
                         (shift PEEK_AT_CONTEXT             TX_INIT_CONTEXT_INITIALIZATION_ROW_OFFSET )
                         (shift PEEK_AT_TRANSACTION         TX_INIT_TRANSACTION_ROW_OFFSET            ))
                      5))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                            ;;
;;   X.3 Common constraints   ;;
;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (tx-initialization---wei-cost-for-sender) (shift (+ transaction/VALUE
                                               (* transaction/GAS_PRICE transaction/GAS_LIMIT))
                                            TX_INIT_TRANSACTION_ROW_OFFSET))

;; sender account operation
;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   tx-initialization---setting-sender-account-row                        (:guard (tx-init---standard-precondition))
                 (begin
                   (eq!     (shift account/ADDRESS_HI             TX_INIT_SENDER_ACCOUNT_ROW_OFFSET)     (shift transaction/FROM_ADDRESS_HI TX_INIT_TRANSACTION_ROW_OFFSET))
                   (eq!     (shift account/ADDRESS_LO             TX_INIT_SENDER_ACCOUNT_ROW_OFFSET)     (shift transaction/FROM_ADDRESS_LO TX_INIT_TRANSACTION_ROW_OFFSET))
                   (account-decrement-balance-by                  TX_INIT_SENDER_ACCOUNT_ROW_OFFSET      (tx-initialization---wei-cost-for-sender))
                   (account-increment-nonce                       TX_INIT_SENDER_ACCOUNT_ROW_OFFSET)
                   (account-same-code                             TX_INIT_SENDER_ACCOUNT_ROW_OFFSET)
                   (account-same-deployment-number-and-status     TX_INIT_SENDER_ACCOUNT_ROW_OFFSET)
                   (account-turn-on-warmth                        TX_INIT_SENDER_ACCOUNT_ROW_OFFSET)
                   (account-same-marked-for-selfdestruct          TX_INIT_SENDER_ACCOUNT_ROW_OFFSET)
                   (account-isnt-precompile                       TX_INIT_SENDER_ACCOUNT_ROW_OFFSET)
                   (DOM-SUB-stamps---standard                     TX_INIT_SENDER_ACCOUNT_ROW_OFFSET
                                                                  0)))

(defconstraint   tx-initialization---EIP-3607---reject-transactions-from-senders-with-deployed-codey          (:guard (tx-init---standard-precondition))
                 (vanishes!    (shift    account/HAS_CODE    TX_INIT_SENDER_ACCOUNT_ROW_OFFSET)))

;; recipient account operation
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   tx-initialization---setting-recipient-account-row                     (:guard (tx-init---standard-precondition))
                 (begin
                   (eq!     (shift account/ADDRESS_HI             TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET)     (shift transaction/TO_ADDRESS_HI     TX_INIT_TRANSACTION_ROW_OFFSET))
                   (eq!     (shift account/ADDRESS_LO             TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET)     (shift transaction/TO_ADDRESS_LO     TX_INIT_TRANSACTION_ROW_OFFSET))
                   (account-increment-balance-by                  TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET      (shift transaction/VALUE             TX_INIT_TRANSACTION_ROW_OFFSET))
                   ;; (account-increment-nonce                       TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET)  ;; message call tx vs deployment tx dependent
                   ;; (account-same-code                             TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET)  ;; message call tx vs deployment tx dependent
                   ;; (account-same-deployment-number-and-status     TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET)  ;; message call tx vs deployment tx dependent
                   (account-turn-on-warmth                        TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET)
                   (account-same-marked-for-selfdestruct          TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET)
                   (account-isnt-precompile                       TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET)
                   (account-retrieve-code-fragment-index          TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET)
                   (DOM-SUB-stamps---standard                     TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET
                                                                  1)))

(defun (tx-initialization---is-deployment)       (force-bin (shift transaction/IS_DEPLOYMENT               TX_INIT_TRANSACTION_ROW_OFFSET)))

;; message call case

(defconstraint   tx-initialization---setting-recipient-account-row-nonce-code-and-deployment-status-message-call-tx     (:guard (tx-init---standard-precondition))
                 (if-zero (tx-initialization---is-deployment)
                          ;; deployment ≡ 0 i.e. smart contract call
                          (begin
                            (account-same-nonce                                        TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET)
                            (account-same-code                                         TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET)
                            (account-same-deployment-number-and-status                 TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET))))

(defconstraint   tx-initialization---setting-recipient-account-row-address-trimming-for-message-call-transaction     (:guard (tx-init---standard-precondition))
                 (if-zero (tx-initialization---is-deployment)
                          ;; deployment ≡ 0 i.e. smart contract call
                          ;; trimming address
                          (account-trim-address   TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET
                                                  (shift    transaction/TO_ADDRESS_HI    TX_INIT_TRANSACTION_ROW_OFFSET)
                                                  (shift    transaction/TO_ADDRESS_LO    TX_INIT_TRANSACTION_ROW_OFFSET))))

;; deployment case

(defconstraint   tx-initialization---setting-recipient-account-row-nonce-deployment-tx       (:guard (tx-init---standard-precondition))
                 (if-not-zero (tx-initialization---is-deployment)
                              ;; deployment ≡ 1 i.e. nontrivial deployments
                              (begin
                                ;; nonce
                                (account-increment-nonce                                   TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET)
                                (debug (vanishes! (shift account/NONCE                     TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET))))))

(defconstraint   tx-initialization---setting-recipient-account-row-code-deployment-tx       (:guard (tx-init---standard-precondition))
                 (if-not-zero (tx-initialization---is-deployment)
                              ;; deployment ≡ 1 i.e. nontrivial deployments
                              (begin
                                ;; code
                                ;; ;; current code
                                (debug (eq! (shift account/HAS_CODE                        TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET) 0))
                                (debug (eq! (shift account/CODE_HASH_HI                    TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET) EMPTY_KECCAK_HI))
                                (debug (eq! (shift account/CODE_HASH_LO                    TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET) EMPTY_KECCAK_LO))
                                (debug (eq! (shift account/CODE_SIZE                       TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET) 0))
                                ;; ;; updated code
                                (eq!        (shift account/HAS_CODE_NEW                    TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET) 0)
                                (debug (eq! (shift account/CODE_HASH_HI                    TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET) EMPTY_KECCAK_HI))
                                (debug (eq! (shift account/CODE_HASH_LO                    TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET) EMPTY_KECCAK_LO))
                                (eq!        (shift account/CODE_SIZE                       TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET)
                                            (shift transaction/INIT_CODE_SIZE              TX_INIT_TRANSACTION_ROW_OFFSET)))))

(defconstraint   tx-initialization---setting-recipient-account-row-deployment-number-and-status-for-deployment-tx       (:guard (tx-init---standard-precondition))
                 (if-not-zero (tx-initialization---is-deployment)
                              ;; deployment ≡ 1 i.e. nontrivial deployments
                              (begin
                                ;; deployment
                                (account-increment-deployment-number                       TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET)
                                (debug (eq! (shift account/DEPLOYMENT_STATUS               TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET) 0))
                                (eq!        (shift account/DEPLOYMENT_STATUS_NEW           TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET) 1))))

;; miscellaneous row
;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;


(defun (tx-initialization---call-data-context-number) (*     HUB_STAMP
                                                 (shift   transaction/COPY_TXCD    TX_INIT_TRANSACTION_ROW_OFFSET)))

(defun (tx-initialization---call-data-size) (shift   transaction/CALL_DATA_SIZE    TX_INIT_TRANSACTION_ROW_OFFSET))

(defconstraint   tx-initialization---setting-miscellaneous-row-flags                           (:guard (tx-init---standard-precondition))
                 (eq! (weighted-MISC-flag-sum              TX_INIT_MISCELLANEOUS_ROW_OFFSET)
                      (* MISC_WEIGHT_MMU
                         (shift transaction/COPY_TXCD      TX_INIT_TRANSACTION_ROW_OFFSET))))

(defconstraint   tx-initialization---copying-transaction-call-data                             (:guard (tx-init---standard-precondition))
                 (if-not-zero    (shift misc/MMU_FLAG      TX_INIT_MISCELLANEOUS_ROW_OFFSET)
                                 (set-MMU-instruction---exo-to-ram-transplants    TX_INIT_MISCELLANEOUS_ROW_OFFSET       ;; offset
                                                                                  ABS_TX_NUM                             ;; source ID
                                                                                  (tx-initialization---call-data-context-number)     ;; target ID
                                                                                  ;; aux_id                                 ;; auxiliary ID
                                                                                  ;; src_offset_hi                          ;; source offset high
                                                                                  ;; src_offset_lo                          ;; source offset low
                                                                                  ;; tgt_offset_lo                          ;; target offset low
                                                                                  (tx-initialization---call-data-size)               ;; size
                                                                                  ;; ref_offset                             ;; reference offset
                                                                                  ;; ref_size                               ;; reference size
                                                                                  ;; success_bit                            ;; success bit
                                                                                  ;; limb_1                                 ;; limb 1
                                                                                  ;; limb_2                                 ;; limb 2
                                                                                  EXO_SUM_WEIGHT_TXCD                    ;; weighted exogenous module flag sum
                                                                                  RLP_TXN_PHASE_DATA                     ;; phase
                                                                                  )))

(defconstraint   tx-initialization---initializing-context                                 (:guard (tx-init---standard-precondition))
                 (begin
                   (initialize-context
                     TX_INIT_CONTEXT_INITIALIZATION_ROW_OFFSET                                                ;; row offset
                     CONTEXT_NUMBER_NEW                                                                       ;; context number
                     0                                                                                        ;; call stack depth
                     1                                                                                        ;; is root
                     0                                                                                        ;; is static
                     (shift     transaction/TO_ADDRESS_HI         TX_INIT_TRANSACTION_ROW_OFFSET)             ;; account address high
                     (shift     transaction/TO_ADDRESS_LO         TX_INIT_TRANSACTION_ROW_OFFSET)             ;; account address low
                     (shift     account/DEPLOYMENT_NUMBER_NEW     TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET)       ;; account deployment number
                     (shift     transaction/TO_ADDRESS_HI         TX_INIT_TRANSACTION_ROW_OFFSET)             ;; byte code address high
                     (shift     transaction/TO_ADDRESS_LO         TX_INIT_TRANSACTION_ROW_OFFSET)             ;; byte code address low
                     (shift     account/DEPLOYMENT_NUMBER_NEW     TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET)       ;; byte code deployment number
                     (shift     account/DEPLOYMENT_STATUS_NEW     TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET)       ;; byte code deployment status
                     (shift     account/CODE_FRAGMENT_INDEX       TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET)       ;; byte code code fragment index
                     (shift     transaction/FROM_ADDRESS_HI       TX_INIT_TRANSACTION_ROW_OFFSET)             ;; caller address high
                     (shift     transaction/FROM_ADDRESS_LO       TX_INIT_TRANSACTION_ROW_OFFSET)             ;; caller address low
                     (shift     transaction/VALUE                 TX_INIT_TRANSACTION_ROW_OFFSET)             ;; call value
                     (tx-initialization---call-data-context-number)                                           ;; caller context
                     0                                                                                        ;; call data offset
                     (tx-initialization---call-data-size)                                                     ;; call data size
                     0                                                                                        ;; return at offset
                     0                                                                                        ;; return at capacity
                     )
                   (debug (eq! CONTEXT_NUMBER_NEW (+ 1 HUB_STAMP)))))


(defconstraint   tx-initialization---transaction-row-partially-justifying-requires-evm-execution           (:guard (tx-init---standard-precondition))
                 (begin
                   (eq!       (shift             transaction/REQUIRES_EVM_EXECUTION       TX_INIT_TRANSACTION_ROW_OFFSET) 1)
                   (if-zero   (shift             transaction/IS_DEPLOYMENT                TX_INIT_TRANSACTION_ROW_OFFSET)
                              (eq!               (shift account/HAS_CODE                  TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET) 1)
                              (is-not-zero!      (shift transaction/INIT_CODE_SIZE        TX_INIT_TRANSACTION_ROW_OFFSET)))))

;; REFUNDS cannot be set at the present time

(defconstraint   tx-initialization---transaction-row-justifying-initial-balance            (:guard (tx-init---standard-precondition))
                 (eq!   (shift   account/BALANCE                               TX_INIT_SENDER_ACCOUNT_ROW_OFFSET)
                        (shift   transaction/INITIAL_BALANCE                   TX_INIT_TRANSACTION_ROW_OFFSET)))

(defconstraint   tx-initialization---transaction-row-justifying-status-code                (:guard (tx-init---standard-precondition))
                 (eq!   (shift   transaction/STATUS_CODE               TX_INIT_TRANSACTION_ROW_OFFSET)
                        (- 1 (shift CONTEXT_WILL_REVERT           (+ 1 TX_INIT_TRANSACTION_ROW_OFFSET)))))

(defconstraint   tx-initialization---transaction-row-justifying-nonce                      (:guard (tx-init---standard-precondition))
                 (eq!   (shift   transaction/NONCE                     TX_INIT_TRANSACTION_ROW_OFFSET)
                        (shift   account/NONCE                         TX_INIT_SENDER_ACCOUNT_ROW_OFFSET)))

;; LEFTOVER_GAS cannot be set at the present time

(defconstraint   tx-initialization---first-row-of-next-context-initializing-some-variables         (:guard (tx-init---standard-precondition))
                 (first-row-of-new-context (+ 1 TX_INIT_TRANSACTION_ROW_OFFSET)                                               ;; row offset
                                           0                                                                                  ;; next caller context number
                                           (shift   account/CODE_FRAGMENT_INDEX                TX_INIT_RECIPIENT_ACCOUNT_ROW_OFFSET)      ;; next CFI
                                           (shift   transaction/GAS_INITIALLY_AVAILABLE        TX_INIT_TRANSACTION_ROW_OFFSET      )      ;; initially available gas
                                           ))
