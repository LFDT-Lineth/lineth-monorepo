(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                                          ;;;;
;;;;    X.5 Account consistency constraints   ;;;;
;;;;                                          ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                          ;;
;;    X.5.1 Properties of the permutation   ;;
;;    X.5.2 Permuted columns                ;;
;;                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defpermutation
    ;; permuted columns
    ;; replace acp with account_consistency_permutation
    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
    (
      acp_PEEK_AT_ACCOUNT
      acp_ADDRESS_HI
      acp_ADDRESS_LO
      acp_DOM_STAMP
      acp_SUB_STAMP
      ;;
      acp_TRM_FLAG
      acp_IS_PRECOMPILE
      acp_NONCE
      acp_NONCE_NEW
      acp_BALANCE
      acp_BALANCE_NEW
      acp_WARMTH
      acp_WARMTH_NEW
      acp_CODE_SIZE
      acp_CODE_SIZE_NEW
      acp_CODE_HASH_HI
      acp_CODE_HASH_LO
      acp_CODE_HASH_HI_NEW
      acp_CODE_HASH_LO_NEW
      acp_DEPLOYMENT_NUMBER
      acp_DEPLOYMENT_NUMBER_NEW
      acp_DEPLOYMENT_NUMBER_INFTY
      acp_DEPLOYMENT_STATUS
      acp_DEPLOYMENT_STATUS_NEW
      ;; acp_DEPLOYMENT_STATUS_INFTY
      acp_MARKED_FOR_SELFDESTRUCT
      acp_MARKED_FOR_SELFDESTRUCT_NEW
      acp_ABS_TX_NUM
    )
    ;; original columns
    ;;;;;;;;;;;;;;;;;;;
    (
      (↓ PEEK_AT_ACCOUNT )
      (↓ account/ADDRESS_HI )
      (↓ account/ADDRESS_LO )
      (↓ DOM_STAMP )
      (↑ SUB_STAMP )
      ;;
      account/TRM_FLAG
      account/IS_PRECOMPILE
      account/NONCE
      account/NONCE_NEW
      account/BALANCE
      account/BALANCE_NEW
      account/WARMTH
      account/WARMTH_NEW
      account/CODE_SIZE
      account/CODE_SIZE_NEW
      account/CODE_HASH_HI
      account/CODE_HASH_LO
      account/CODE_HASH_HI_NEW
      account/CODE_HASH_LO_NEW
      account/DEPLOYMENT_NUMBER
      account/DEPLOYMENT_NUMBER_NEW
      account/DEPLOYMENT_NUMBER_INFTY
      account/DEPLOYMENT_STATUS
      account/DEPLOYMENT_STATUS_NEW
      ;; account/DEPLOYMENT_STATUS_INFTY
      account/MARKED_FOR_SELFDESTRUCT
      account/MARKED_FOR_SELFDESTRUCT_NEW
      ABS_TX_NUM
    )
  )

;; we are guaranteed that this is a 20B integer
(defun (acp_full_address) (+ (* (^ 256 16) acp_ADDRESS_HI)
                             acp_ADDRESS_LO))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                    ;;
;;    X.5.3 Constraints for acc_FIRST and acc_FINAL   ;;
;;                                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint account-consistency-FINAL-FIRST-generalities ()
               (begin
                 (debug (is-binary acc_FIRST ))
                 (debug (is-binary acc_FINAL ))
                 (if-zero (force-bool acp_PEEK_AT_ACCOUNT)
                          (vanishes! (+ acc_FIRST
                                        acc_FINAL)))))

(defconstraint account-consistency-FINAL-FIRST-first-account-row ()
               (if-not-zero (- 1 (prev acp_PEEK_AT_ACCOUNT))
                            (if-not-zero acp_PEEK_AT_ACCOUNT
                                         (eq! acc_FIRST 1))))

(defconstraint account-consistency-FINAL-FIRST-repeat-encounter ()
               (if-not-zero (prev acp_PEEK_AT_ACCOUNT)
                            (if-not-zero acp_PEEK_AT_ACCOUNT
                                         (if-eq-else (acp_full_address) (prev (acp_full_address))
                                                     (eq! (+ acc_FIRST (prev acc_FINAL)) 0)
                                                     (eq! (+ acc_FIRST (prev acc_FINAL)) 2)))))

(defconstraint account-consistency-FINAL-FIRST-final-row-1 ()
               (if-not-zero (prev acp_PEEK_AT_ACCOUNT)
                            (if-not-zero (- 1 acp_PEEK_AT_ACCOUNT)
                                         (eq! (prev acc_FINAL) 1))))

(defconstraint account-consistency-FINAL-FIRST-final-row-2 (:domain {-1})
               (if-not-zero acp_PEEK_AT_ACCOUNT
                            (eq! acc_FINAL 1)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                        ;;
;;    X.5.4 Constraints   ;;
;;                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint account-consistency-initialization ()
               (if-not-zero acc_FIRST
                            (begin
                              (eq! acp_TRM_FLAG 1)
                              (eq! acp_WARMTH acp_IS_PRECOMPILE)
                              (vanishes! acp_MARKED_FOR_SELFDESTRUCT)
                              (vanishes! acp_DEPLOYMENT_NUMBER)
                              (vanishes! acp_DEPLOYMENT_STATUS))))

(defconstraint account-consistency-simple-linking-constraints ()
               (if-not-zero acp_PEEK_AT_ACCOUNT
                            (if-zero acc_FIRST
                                     (begin
                                       (was-eq! acp_NONCE_NEW               acp_NONCE                   )
                                       (was-eq! acp_BALANCE_NEW             acp_BALANCE                 )
                                       (was-eq! acp_CODE_SIZE_NEW           acp_CODE_SIZE               )
                                       (was-eq! acp_CODE_HASH_HI_NEW        acp_CODE_HASH_HI            )
                                       (was-eq! acp_CODE_HASH_LO_NEW        acp_CODE_HASH_LO            )
                                       (was-eq! acp_IS_PRECOMPILE           acp_IS_PRECOMPILE           )
                                       (was-eq! acp_DEPLOYMENT_NUMBER_NEW   acp_DEPLOYMENT_NUMBER       )
                                       (was-eq! acp_DEPLOYMENT_STATUS_NEW   acp_DEPLOYMENT_STATUS       )
                                       (was-eq! acp_DEPLOYMENT_NUMBER_INFTY acp_DEPLOYMENT_NUMBER_INFTY )
                                       ;; (was-eq! acp_DEPLOYMENT_STATUS_INFTY acp_DEPLOYMENT_STATUS_INFTY )
                                       ))))

(defconstraint account-consistency-linking-and-resetting-constraints ()
               (if-not-zero acp_PEEK_AT_ACCOUNT
                            (if-zero acc_FIRST
                                     (if-eq-else acp_ABS_TX_NUM (prev acp_ABS_TX_NUM)
                                                 (begin
                                                   (was-eq! acp_WARMTH_NEW                    acp_WARMTH)
                                                   (was-eq! acp_MARKED_FOR_SELFDESTRUCT_NEW acp_MARKED_FOR_SELFDESTRUCT))
                                                 (begin
                                                   (eq!       acp_WARMTH acp_IS_PRECOMPILE)
                                                   (vanishes! acp_MARKED_FOR_SELFDESTRUCT)
                                                   (debug (vanishes! acp_DEPLOYMENT_STATUS)))))))

;; I really doubt we need the final deployment status
;; it should necessarily be 0 ... if things are done right
(defconstraint account-consistency-finalization-constraints ()
               (if-not-zero acc_FINAL
                            (begin
                              (eq! acp_DEPLOYMENT_NUMBER_INFTY acp_DEPLOYMENT_NUMBER)
                              ;; (eq! acp_DEPLOYMENT_STATUS_INFTY acp_DEPLOYMENT_STATUS)
                              )))

(defconstraint account-consistency-monotony-constraints ()
               (if-not-zero acp_PEEK_AT_ACCOUNT
                            (begin
                              (any! (eq! acp_DEPLOYMENT_NUMBER_NEW acp_DEPLOYMENT_NUMBER)
                                    (eq! acp_DEPLOYMENT_NUMBER_NEW (+ 1 acp_DEPLOYMENT_NUMBER)))
                              (if-not-zero acp_MARKED_FOR_SELFDESTRUCT
                                           (eq! acp_MARKED_FOR_SELFDESTRUCT_NEW 1)))))
