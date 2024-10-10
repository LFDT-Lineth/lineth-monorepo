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
  tx-init---row-offset---sender-account-row            0
  tx-init---row-offset---recipient-account-row         1
  tx-init---row-offset---miscellaneous-row             2
  tx-init---row-offset---context-initialization-row    3
  tx-init---row-offset---transaction-row               4
  tx-init---number-of-rows                             5
  )

(defconstraint   tx-initialization---setting-the-peeking-flags (:guard (tx-init---standard-precondition))
                 (eq!    (+    (shift PEEK_AT_ACCOUNT             tx-init---row-offset---sender-account-row         )
                               (shift PEEK_AT_ACCOUNT             tx-init---row-offset---recipient-account-row      )
                               (shift PEEK_AT_MISCELLANEOUS       tx-init---row-offset---miscellaneous-row          )
                               (shift PEEK_AT_CONTEXT             tx-init---row-offset---context-initialization-row )
                               (shift PEEK_AT_TRANSACTION         tx-init---row-offset---transaction-row            ))
                         tx-init---number-of-rows))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                            ;;
;;   X.3 Common constraints   ;;
;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (tx-initialization---wei-cost-for-sender) (shift (+ transaction/VALUE
                                               (* transaction/GAS_PRICE transaction/GAS_LIMIT))
                                            tx-init---row-offset---transaction-row))

;; sender account operation
;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   tx-initialization---setting-sender-account-row                        (:guard (tx-init---standard-precondition))
                 (begin
                   (eq!     (shift account/ADDRESS_HI             tx-init---row-offset---sender-account-row)     (shift transaction/FROM_ADDRESS_HI tx-init---row-offset---transaction-row))
                   (eq!     (shift account/ADDRESS_LO             tx-init---row-offset---sender-account-row)     (shift transaction/FROM_ADDRESS_LO tx-init---row-offset---transaction-row))
                   (account-decrement-balance-by                  tx-init---row-offset---sender-account-row      (tx-initialization---wei-cost-for-sender))
                   (account-increment-nonce                       tx-init---row-offset---sender-account-row)
                   (account-same-code                             tx-init---row-offset---sender-account-row)
                   (account-same-deployment-number-and-status     tx-init---row-offset---sender-account-row)
                   (account-turn-on-warmth                        tx-init---row-offset---sender-account-row)
                   (account-same-marked-for-selfdestruct          tx-init---row-offset---sender-account-row)
                   (account-isnt-precompile                       tx-init---row-offset---sender-account-row)
                   (DOM-SUB-stamps---standard                     tx-init---row-offset---sender-account-row
                                                                  0)))

(defconstraint   tx-initialization---EIP-3607---reject-transactions-from-senders-with-deployed-codey          (:guard (tx-init---standard-precondition))
                 (vanishes!    (shift    account/HAS_CODE    tx-init---row-offset---sender-account-row)))

;; recipient account operation
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   tx-initialization---setting-recipient-account-row                     (:guard (tx-init---standard-precondition))
                 (begin
                   (eq!     (shift account/ADDRESS_HI             tx-init---row-offset---recipient-account-row)     (shift transaction/TO_ADDRESS_HI     tx-init---row-offset---transaction-row))
                   (eq!     (shift account/ADDRESS_LO             tx-init---row-offset---recipient-account-row)     (shift transaction/TO_ADDRESS_LO     tx-init---row-offset---transaction-row))
                   (account-increment-balance-by                  tx-init---row-offset---recipient-account-row      (shift transaction/VALUE             tx-init---row-offset---transaction-row))
                   ;; (account-increment-nonce                       tx-init---row-offset---recipient-account-row)  ;; message call tx vs deployment tx dependent
                   ;; (account-same-code                             tx-init---row-offset---recipient-account-row)  ;; message call tx vs deployment tx dependent
                   ;; (account-same-deployment-number-and-status     tx-init---row-offset---recipient-account-row)  ;; message call tx vs deployment tx dependent
                   (account-turn-on-warmth                        tx-init---row-offset---recipient-account-row)
                   (account-same-marked-for-selfdestruct          tx-init---row-offset---recipient-account-row)
                   (account-isnt-precompile                       tx-init---row-offset---recipient-account-row)
                   (account-retrieve-code-fragment-index          tx-init---row-offset---recipient-account-row)
                   (DOM-SUB-stamps---standard                     tx-init---row-offset---recipient-account-row
                                                                  1)))

(defun (tx-initialization---is-deployment)       (force-bin (shift transaction/IS_DEPLOYMENT               tx-init---row-offset---transaction-row)))

;; message call case

(defconstraint   tx-initialization---recipient-account-row---message-call-tx---nonce-code-and-deployment-status     (:guard (tx-init---standard-precondition))
                 (if-zero (tx-initialization---is-deployment)
                          ;; deployment ≡ 0 i.e. smart contract call
                          (begin
                            (account-same-nonce                                        tx-init---row-offset---recipient-account-row)
                            (account-same-code                                         tx-init---row-offset---recipient-account-row)
                            (account-same-deployment-number-and-status                 tx-init---row-offset---recipient-account-row))))

(defconstraint   tx-initialization---recipient-account-row---message-call-tx---address-trimming     (:guard (tx-init---standard-precondition))
                 (if-zero (tx-initialization---is-deployment)
                          ;; deployment ≡ 0 i.e. smart contract call
                          ;; trimming address
                          (account-trim-address   tx-init---row-offset---recipient-account-row
                                                  (shift    transaction/TO_ADDRESS_HI    tx-init---row-offset---transaction-row)
                                                  (shift    transaction/TO_ADDRESS_LO    tx-init---row-offset---transaction-row))))

;; deployment case

(defconstraint   tx-initialization---recipient-account-row---deployment-transaction---nonce       (:guard (tx-init---standard-precondition))
                 (if-not-zero (tx-initialization---is-deployment)
                              ;; deployment ≡ 1 i.e. nontrivial deployments
                              (begin
                                ;; nonce
                                (account-increment-nonce                                   tx-init---row-offset---recipient-account-row)
                                (debug (vanishes! (shift account/NONCE                     tx-init---row-offset---recipient-account-row))))))

(defconstraint   tx-initialization---recipient-account-row---deployment-transaction---code       (:guard (tx-init---standard-precondition))
                 (if-not-zero (tx-initialization---is-deployment)
                              ;; deployment ≡ 1 i.e. nontrivial deployments
                              (begin
                                ;; code
                                ;; current code
                                (vanishes!      (shift account/HAS_CODE               tx-init---row-offset---recipient-account-row))
                                (debug     (eq! (shift account/CODE_HASH_HI           tx-init---row-offset---recipient-account-row) EMPTY_KECCAK_HI))
                                (debug     (eq! (shift account/CODE_HASH_LO           tx-init---row-offset---recipient-account-row) EMPTY_KECCAK_LO))
                                (vanishes!      (shift account/CODE_SIZE              tx-init---row-offset---recipient-account-row))
                                ;; updated code
                                (vanishes!      (shift account/HAS_CODE_NEW           tx-init---row-offset---recipient-account-row))
                                (debug     (eq! (shift account/CODE_HASH_HI           tx-init---row-offset---recipient-account-row) EMPTY_KECCAK_HI))
                                (debug     (eq! (shift account/CODE_HASH_LO           tx-init---row-offset---recipient-account-row) EMPTY_KECCAK_LO))
                                (eq!            (shift account/CODE_SIZE              tx-init---row-offset---recipient-account-row)
                                                (shift transaction/INIT_CODE_SIZE     tx-init---row-offset---transaction-row)))))

(defconstraint   tx-initialization---recipient-account-row---deployment-transaction---deployment-number-and-status       (:guard (tx-init---standard-precondition))
                 (if-not-zero (tx-initialization---is-deployment)
                              ;; deployment ≡ 1 i.e. nontrivial deployments
                              (begin
                                ;; deployment
                                (account-increment-deployment-number                       tx-init---row-offset---recipient-account-row)
                                (debug (eq! (shift account/DEPLOYMENT_STATUS               tx-init---row-offset---recipient-account-row) 0))
                                (eq!        (shift account/DEPLOYMENT_STATUS_NEW           tx-init---row-offset---recipient-account-row) 1))))

;; miscellaneous row
;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;


(defun (tx-initialization---call-data-context-number) (*     HUB_STAMP
                                                 (shift   transaction/COPY_TXCD    tx-init---row-offset---transaction-row)))

(defun (tx-initialization---call-data-size) (shift   transaction/CALL_DATA_SIZE    tx-init---row-offset---transaction-row))

(defconstraint   tx-initialization---setting-miscellaneous-row-flags                           (:guard (tx-init---standard-precondition))
                 (eq! (weighted-MISC-flag-sum              tx-init---row-offset---miscellaneous-row)
                      (* MISC_WEIGHT_MMU
                         (shift transaction/COPY_TXCD      tx-init---row-offset---transaction-row))))

(defconstraint   tx-initialization---copying-transaction-call-data                             (:guard (tx-init---standard-precondition))
                 (if-not-zero    (shift misc/MMU_FLAG      tx-init---row-offset---miscellaneous-row)
                                 (set-MMU-instruction---exo-to-ram-transplants    tx-init---row-offset---miscellaneous-row        ;; offset
                                                                                  ABS_TX_NUM                                      ;; source ID
                                                                                  (tx-initialization---call-data-context-number)  ;; target ID
                                                                                  ;; aux_id                                          ;; auxiliary ID
                                                                                  ;; src_offset_hi                                   ;; source offset high
                                                                                  ;; src_offset_lo                                   ;; source offset low
                                                                                  ;; tgt_offset_lo                                   ;; target offset low
                                                                                  (tx-initialization---call-data-size)            ;; size
                                                                                  ;; ref_offset                                      ;; reference offset
                                                                                  ;; ref_size                                        ;; reference size
                                                                                  ;; success_bit                                     ;; success bit
                                                                                  ;; limb_1                                          ;; limb 1
                                                                                  ;; limb_2                                          ;; limb 2
                                                                                  EXO_SUM_WEIGHT_TXCD                             ;; weighted exogenous module flag sum
                                                                                  RLP_TXN_PHASE_DATA                              ;; phase
                                                                                  )))

(defconstraint   tx-initialization---initializing-context                                 (:guard (tx-init---standard-precondition))
                 (begin
                   (initialize-context
                     tx-init---row-offset---context-initialization-row                                                ;; row offset
                     CONTEXT_NUMBER_NEW                                                                               ;; context number
                     0                                                                                                ;; call stack depth
                     1                                                                                                ;; is root
                     0                                                                                                ;; is static
                     (shift     transaction/TO_ADDRESS_HI         tx-init---row-offset---transaction-row)             ;; account address high
                     (shift     transaction/TO_ADDRESS_LO         tx-init---row-offset---transaction-row)             ;; account address low
                     (shift     account/DEPLOYMENT_NUMBER_NEW     tx-init---row-offset---recipient-account-row)       ;; account deployment number
                     (shift     transaction/TO_ADDRESS_HI         tx-init---row-offset---transaction-row)             ;; byte code address high
                     (shift     transaction/TO_ADDRESS_LO         tx-init---row-offset---transaction-row)             ;; byte code address low
                     (shift     account/DEPLOYMENT_NUMBER_NEW     tx-init---row-offset---recipient-account-row)       ;; byte code deployment number
                     (shift     account/DEPLOYMENT_STATUS_NEW     tx-init---row-offset---recipient-account-row)       ;; byte code deployment status
                     (shift     account/CODE_FRAGMENT_INDEX       tx-init---row-offset---recipient-account-row)       ;; byte code code fragment index
                     (shift     transaction/FROM_ADDRESS_HI       tx-init---row-offset---transaction-row)             ;; caller address high
                     (shift     transaction/FROM_ADDRESS_LO       tx-init---row-offset---transaction-row)             ;; caller address low
                     (shift     transaction/VALUE                 tx-init---row-offset---transaction-row)             ;; call value
                     (tx-initialization---call-data-context-number)                                                   ;; caller context
                     0                                                                                                ;; call data offset
                     (tx-initialization---call-data-size)                                                             ;; call data size
                     0                                                                                                ;; return at offset
                     0                                                                                                ;; return at capacity
                     )
                   (debug (eq! CONTEXT_NUMBER_NEW (+ 1 HUB_STAMP)))))


(defconstraint   tx-initialization---transaction-row-partially-justifying-requires-evm-execution           (:guard (tx-init---standard-precondition))
                 (begin
                   (eq!       (shift             transaction/REQUIRES_EVM_EXECUTION       tx-init---row-offset---transaction-row) 1)
                   (if-zero   (shift             transaction/IS_DEPLOYMENT                tx-init---row-offset---transaction-row)
                              (eq!               (shift account/HAS_CODE                  tx-init---row-offset---recipient-account-row) 1)
                              (is-not-zero!      (shift transaction/INIT_CODE_SIZE        tx-init---row-offset---transaction-row)))))

;; REFUNDS cannot be set at the present time

(defconstraint   tx-initialization---transaction-row-justifying-initial-balance            (:guard (tx-init---standard-precondition))
                 (eq!   (shift   account/BALANCE                               tx-init---row-offset---sender-account-row)
                        (shift   transaction/INITIAL_BALANCE                   tx-init---row-offset---transaction-row)))

(defconstraint   tx-initialization---transaction-row-justifying-status-code                (:guard (tx-init---standard-precondition))
                 (eq!   (shift   transaction/STATUS_CODE               tx-init---row-offset---transaction-row)
                        (- 1 (shift CONTEXT_WILL_REVERT           (+ 1 tx-init---row-offset---transaction-row)))))

(defconstraint   tx-initialization---transaction-row-justifying-nonce                      (:guard (tx-init---standard-precondition))
                 (eq!   (shift   transaction/NONCE                     tx-init---row-offset---transaction-row)
                        (shift   account/NONCE                         tx-init---row-offset---sender-account-row)))

;; LEFTOVER_GAS cannot be set at the present time

(defconstraint   tx-initialization---first-row-of-next-context-initializing-some-variables         (:guard (tx-init---standard-precondition))
                 (first-row-of-new-context (+ 1 tx-init---row-offset---transaction-row)                                                           ;; row offset
                                           0                                                                                                      ;; next caller context number
                                           (shift   account/CODE_FRAGMENT_INDEX                tx-init---row-offset---recipient-account-row)      ;; next CFI
                                           (shift   transaction/GAS_INITIALLY_AVAILABLE        tx-init---row-offset---transaction-row      )      ;; initially available gas
                                           ))
