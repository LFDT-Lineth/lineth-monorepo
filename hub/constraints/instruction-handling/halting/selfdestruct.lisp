(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                       ;;;;
;;;;    X.5 SELFDESTRUCT   ;;;;
;;;;                       ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;    X.5.1 Introduction     ;;
;;    X.5.2 Representation   ;;
;;    X.5.3 Scenario         ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; TODO: uncomment
(defconstraint setting-selfdestruct-scenario-sum ()
               (if-not-zero PEEK_AT_STACK
                            (if-not-zero stack/HALT_FLAG
                                         (if-not-zero [stack/DEC_FLAG 4]
                                                      (if-not-zero (- 1 stack/SUX stack/SOX)
                                                                   (begin
                                                                     (will-eq! PEEK_AT_SCENARIO                        1)
                                                                     (will-eq! (scenario-shorthand-SELFDESTRUCT-sum)   1)))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    X.5.4 Shorthands   ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconst
  ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_CONTEXT_ROW              1
  ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW              2
  ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW             3
  ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW      4
  ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_UNDOING_ROW     5
  ROW_OFFSET_FOR_SELFDESTRUCT_ACCOUNT_DELETION_ROW           4
  )

;; TODO: uncomment
(defun (selfdestruct-raw-recipient-address-hi)  (shift [stack/STACK_ITEM_VALUE_HI 1] -1))
(defun (selfdestruct-raw-recipient-address-lo)  (shift [stack/STACK_ITEM_VALUE_LO 1] -1))
;;
(defun (selfdestruct-is-static)          (shift context/IS_STATIC                   ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_CONTEXT_ROW))
(defun (selfdestruct-is-deployment)      (shift context/BYTE_CODE_DEPLOYMENT_STATUS ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_CONTEXT_ROW))
(defun (selfdestruct-account-address-hi) (shift context/ACCOUNT_ADDRESS_HI          ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_CONTEXT_ROW))
(defun (selfdestruct-account-address-lo) (shift context/ACCOUNT_ADDRESS_LO          ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_CONTEXT_ROW))
(defun (selfdestruct-account-address)    (+ (* (^ 256 LLARGE) (selfdestruct-account-address-hi))
                                            (selfdestruct-account-address-lo)))
;;
(defun (selfdestruct-balance)                   (shift account/BALANCE                 ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW))
(defun (selfdestruct-is-marked)                 (shift account/MARKED_FOR_SELFDESTRUCT ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW))
;;
(defun (selfdestruct-recipient-address-hi)      (shift account/ADDRESS_HI ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW))
(defun (selfdestruct-recipient-address-lo)      (shift account/ADDRESS_LO ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW))
(defun (selfdestruct-recipient-address)         (+ (* (^ 256 LLARGE) (selfdestruct-recipient-address-hi)) (selfdestruct-recipient-address-lo)))
(defun (selfdestruct-recipient-trm-flag)        (shift account/TRM_FLAG ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW))
(defun (selfdestruct-recipient-exists)          (shift account/EXISTS   ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW))
(defun (selfdestruct-recipient-warmth)          (shift account/WARMTH   ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                        ;;
;;    X.5.5 Constraints   ;;
;;                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (selfdestruct-scenario-precondition) (* PEEK_AT_SCENARIO (scenario-shorthand-SELFDESTRUCT-sum)))

(defconstraint selfdestruct-looking-back (:guard (selfdestruct-scenario-precondition))
               (begin
                 (eq! (prev PEEK_AT_STACK) 1)
                 (eq! (prev stack/INSTRUCTION) EVM_INST_SELFDESTRUCT)
                 (eq! XAHOY (prev (+ stack/STATICX stack/OOGX)))))

(defconstraint selfdestruct-setting-stack-pattern (:guard (selfdestruct-scenario-precondition))
               (prev (stack-pattern-1-0)))

(defconstraint selfdestruct-setting-refund (:guard (selfdestruct-scenario-precondition))
               (eq! REFUND_COUNTER_NEW (+ REFUND_COUNTER
                                  (* REFUND_CONST_R_SELFDESTRUCT scenario/SELFDESTRUCT_WONT_REVERT_NOT_YET_MARKED))))

(defconstraint selfdestruct-setting-the-right-scenario (:guard (selfdestruct-scenario-precondition))
               (begin
                 (eq! XAHOY scenario/SELFDESTRUCT_EXCEPTION)
                 (if-zero XAHOY
                          (begin
                            (eq! scenario/SELFDESTRUCT_WILL_REVERT             CONTEXT_WILL_REVERT)
                            (eq! (scenario-shorthand-SELFDESTRUCT-wont-revert) (- 1 CONTEXT_WILL_REVERT))))
                 (if-zero CONTEXT_WILL_REVERT
                          (begin
                            (eq! (scenario-shorthand-SELFDESTRUCT-wont-revert)    1)
                            (eq! scenario/SELFDESTRUCT_WONT_REVERT_ALREADY_MARKED (selfdestruct-is-marked))
                            (eq! scenario/SELFDESTRUCT_WONT_REVERT_NOT_YET_MARKED (- 1 (selfdestruct-is-marked)))))))

(defconstraint selfdestruct-setting-NSR-and-peeking-flags (:guard (selfdestruct-scenario-precondition))
               (begin
                 (if-not-zero (prev stack/STATICX)
                              (begin
                                (eq! (prev NSR) 3)
                                (eq! (prev NSR) (+ (shift PEEK_AT_SCENARIO 0)
                                                   (shift PEEK_AT_CONTEXT  1)
                                                   (shift PEEK_AT_CONTEXT  2)))))
                 (if-not-zero (prev stack/OOGX)
                              (begin
                                (eq! (prev NSR) 5)
                                (eq! (prev NSR) (+ (shift PEEK_AT_SCENARIO 0)
                                                   (shift PEEK_AT_CONTEXT  1)
                                                   (shift PEEK_AT_ACCOUNT  2)
                                                   (shift PEEK_AT_ACCOUNT  3)
                                                   (shift PEEK_AT_CONTEXT  4)))))
                 (if-not-zero scenario/SELFDESTRUCT_WILL_REVERT
                              (begin
                                (eq! (prev NSR) 7)
                                (eq! (prev NSR) (+ (shift PEEK_AT_SCENARIO 0)
                                                   (shift PEEK_AT_CONTEXT  1)
                                                   (shift PEEK_AT_ACCOUNT  2)
                                                   (shift PEEK_AT_ACCOUNT  3)
                                                   (shift PEEK_AT_ACCOUNT  4)
                                                   (shift PEEK_AT_ACCOUNT  5)
                                                   (shift PEEK_AT_CONTEXT  6)))))
                 (if-not-zero scenario/SELFDESTRUCT_WONT_REVERT_ALREADY_MARKED
                              (begin
                                (eq! (prev NSR) 5)
                                (eq! (prev NSR) (+ (shift PEEK_AT_SCENARIO 0)
                                                   (shift PEEK_AT_CONTEXT  1)
                                                   (shift PEEK_AT_ACCOUNT  2)
                                                   (shift PEEK_AT_ACCOUNT  3)
                                                   (shift PEEK_AT_CONTEXT  4)))))
                 (if-not-zero scenario/SELFDESTRUCT_WONT_REVERT_NOT_YET_MARKED
                              (begin
                                (eq! (prev NSR) 6)
                                (eq! (prev NSR) (+ (shift PEEK_AT_SCENARIO 0)
                                                   (shift PEEK_AT_CONTEXT  1)
                                                   (shift PEEK_AT_ACCOUNT  2)
                                                   (shift PEEK_AT_ACCOUNT  3)
                                                   (shift PEEK_AT_ACCOUNT  4)
                                                   (shift PEEK_AT_CONTEXT  5)))))))

(defconstraint selfdestruct-reading-context-data (:guard (selfdestruct-scenario-precondition))
               (read-context-data ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_CONTEXT_ROW       ;; row offset
                                  CONTEXT_NUMBER))                                    ;; context to read

(defconstraint selfdestruct-returning-empty-return-data (:guard (selfdestruct-scenario-precondition))
               (begin
                 (if-not-zero (prev stack/STATICX)                             (execution-provides-empty-return-data 2))
                 (if-not-zero (prev stack/OOGX)                                (execution-provides-empty-return-data 4))
                 (if-not-zero scenario/SELFDESTRUCT_WILL_REVERT                (execution-provides-empty-return-data 6))
                 (if-not-zero scenario/SELFDESTRUCT_WONT_REVERT_ALREADY_MARKED (execution-provides-empty-return-data 4))
                 (if-not-zero scenario/SELFDESTRUCT_WONT_REVERT_NOT_YET_MARKED (execution-provides-empty-return-data 5))))

(defconstraint selfdestruct-justifying-the-static-exception (:guard (selfdestruct-scenario-precondition))
               (eq! (prev stack/STATICX) (selfdestruct-is-static)))

(defconstraint selfdestruct-justifying-the-gas-cost (:guard (selfdestruct-scenario-precondition))
               (if-zero (force-bin (prev stack/STATICX))
                        (if-zero (selfdestruct-balance)
                                 ;; account has zero balance
                                 (eq! GAS_COST
                                      (+ GAS_CONST_G_SELFDESTRUCT
                                         (* (- 1 (selfdestruct-recipient-warmth)) GAS_CONST_G_COLD_ACCOUNT_ACCESS)))
                                 ;; account has nonzero balance
                                 (eq! GAS_COST
                                      (+ GAS_CONST_G_SELFDESTRUCT
                                         (* (- 1 (selfdestruct-recipient-warmth)) GAS_CONST_G_COLD_ACCOUNT_ACCESS)
                                         (* (- 1 (selfdestruct-recipient-exists)) GAS_CONST_G_NEW_ACCOUNT        ))))))

(defconstraint selfdestruct-generalities-for-the-first-account-row (:guard (selfdestruct-scenario-precondition))
               (begin
                 (if-zero     (force-bin (prev stack/STATICX))
                              (begin
                                (debug (vanishes! (shift account/ROM_LEX_FLAG ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW)))
                                (debug (vanishes! (shift account/TRM_FLAG     ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW)))
                                (eq!   (selfdestruct-account-address-hi)  (shift account/ADDRESS_HI ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW))
                                (eq!   (selfdestruct-account-address-lo)  (shift account/ADDRESS_LO ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW))
                                ;; balance
                                (account-same-nonce  ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW)
                                (account-same-warmth ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW)
                                ;; code
                                ;; depoyment
                                ;; selfdestruct marking
                                (standard-dom-sub-stamps ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW
                                                         0                                        )))))

(defconstraint selfdestruct-setting-code-and-deployment-for-the-first-account-row (:guard (selfdestruct-scenario-precondition))
               (if-zero     (force-bin (prev stack/STATICX))
                            (begin 
                              (if-not-zero XAHOY
                                           ;; XAHOY = 1
                                           (begin (account-same-code                             ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW)
                                                  (account-same-deployment-number-and-status     ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW))
                                           ;; XAHOY = 0
                                           (if-zero (force-bin (selfdestruct-is-deployment))
                                                    (begin (account-same-code                             ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW)
                                                           (account-same-deployment-number-and-status     ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW))
                                                    (begin
                                                      (eq!        (shift account/CODE_SIZE_NEW               ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW) 0)
                                                      (eq!        (shift account/CODE_HASH_HI_NEW            ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW) EMPTY_KECCAK_HI)
                                                      (eq!        (shift account/CODE_HASH_LO_NEW            ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW) EMPTY_KECCAK_LO)
                                                      (debug (eq! (shift account/DEPLOYMENT_STATUS           ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW) 1))
                                                      (eq!        (shift account/DEPLOYMENT_STATUS_NEW       ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW) 0)
                                                      (account-same-deployment-number                        ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW)))))))

(defconstraint selfdestruct-setting-balance-and-marked-for-SELFDESTRUCT-for-first-account-row (:guard (selfdestruct-scenario-precondition))
               (if-zero     (force-bin (prev stack/STATICX))
                            (begin
                              (if-not-zero (prev stack/OOGX)
                                           (begin
                                             (account-same-balance                      ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW)
                                             (account-same-marked-for-selfdestruct      ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW)))
                              (if-not-zero (scenario-shorthand-SELFDESTRUCT-unexceptional)     (account-decrement-balance-by              ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW      (selfdestruct-balance)))
                              (if-not-zero scenario/SELFDESTRUCT_WILL_REVERT                   (account-same-marked-for-selfdestruct      ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW))
                              (if-not-zero scenario/SELFDESTRUCT_WONT_REVERT_ALREADY_MARKED    (account-same-marked-for-selfdestruct      ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW))
                              (if-not-zero scenario/SELFDESTRUCT_WONT_REVERT_NOT_YET_MARKED    (account-mark-account-for-selfdestruct     ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW)))))

(defconstraint selfdestruct-generalities-for-the-second-account-row (:guard (selfdestruct-scenario-precondition))
               (begin
                 (if-zero     (force-bin (prev stack/STATICX))
                              (begin
                                ( debug (eq! (shift account/ROM_LEX_FLAG             ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW) 0 ) )
                                (eq!         (shift account/TRM_FLAG                 ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW) 1 )
                                (eq!         (shift account/TRM_RAW_ADDRESS_HI       ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW) (selfdestruct-raw-recipient-address-hi))
                                (eq!         (shift account/ADDRESS_LO               ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW) (selfdestruct-raw-recipient-address-lo))
                                ;; balance
                                (account-same-nonce                                  ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW)
                                ;; warmth
                                (account-same-code                                   ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW)
                                (account-same-deployment-number-and-status           ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW)
                                (account-same-marked-for-selfdestruct                ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW)
                                (standard-dom-sub-stamps                             ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW 1 )))))

(defconstraint selfdestruct-balance-and-warmth-for-second-account-row (:guard (selfdestruct-scenario-precondition))
               (begin
                 (if-not-zero (prev stack/OOGX)
                              (begin
                                (account-same-balance               ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW)
                                (account-same-warmth                ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW)))
                 (if-not-zero (scenario-shorthand-SELFDESTRUCT-unexceptional)
                              (begin
                                (account-turn-on-warmth             ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW)
                                (if-eq-else (selfdestruct-account-address) (selfdestruct-recipient-address)
                                            ;; self destructing account address = recipient address
                                            (begin
                                              (debug (vanishes! account/BALANCE     ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW))
                                              (account-same-balance                 ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW))
                                            ;; self destructing account address ≠ recipient address
                                            (account-increment-balance-by           ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW    (selfdestruct-balance)))))))


;; (defconstraint selfdestruct-returning-empty-return-data (:guard (selfdestruct-scenario-precondition))
;;                (begin
;;                  (if-zero     (force-bin (prev stack/STATICX))
;;                  (if-not-zero (prev stack/STATICX)
;;                  (if-not-zero (prev stack/OOGX)
;;                  (if-not-zero scenario/SELFDESTRUCT_WILL_REVERT
;;                  (if-not-zero scenario/SELFDESTRUCT_WONT_REVERT_ALREADY_MARKED
;;                  (if-not-zero scenario/SELFDESTRUCT_WONT_REVERT_NOT_YET_MARKED


;; (defconstraint selfdestruct- (:guard (selfdestruct-scenario-precondition))
;;                (begin

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                               ;;
;;    X.5.6 Undoing rows for scenario/SELFDESTRUCT_WILL_REVERT   ;;
;;                                                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (selfdestruct-scenario-WILL_REVERT-precondition) (* PEEK_AT_SCENARIO
                                                           scenario/SELFDESTRUCT_WILL_REVERT))

(defconstraint selfdestruct-first-undoing-row-for-WILL_REVERT-scenario (:guard (selfdestruct-scenario-WILL_REVERT-precondition))
               (begin
                 (debug (eq! (shift account/ROM_LEX_FLAG      ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW) 0))
                 (debug (eq! (shift account/TRM_FLAG          ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW) 0))
                 (account-same-address-as                     ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW      ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW)
                 (account-undo-balance-update                 ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW      ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW)
                 (account-undo-nonce-update                   ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW      ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW)
                 (account-undo-warmth-update                  ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW      ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW)
                 (account-undo-code-update                    ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW      ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW)
                 (account-undo-deployment-status-update       ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW      ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW)
                 (account-same-marked-for-selfdestruct        ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW)
                 (revert-dom-sub-stamps                       ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW 2)))

(defconstraint selfdestruct-second-undoing-row-for-WILL_REVERT-scenario (:guard (selfdestruct-scenario-WILL_REVERT-precondition))
               (begin
                 (debug (eq! (shift account/ROM_LEX_FLAG      ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_UNDOING_ROW) 0))
                 (debug (eq! (shift account/TRM_FLAG          ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_UNDOING_ROW) 0))
                 (account-same-address-as                     ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_UNDOING_ROW      ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW)
                 (account-undo-balance-update                 ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_UNDOING_ROW      ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW)
                 (account-undo-nonce-update                   ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_UNDOING_ROW      ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW)
                 (account-undo-warmth-update                  ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_UNDOING_ROW      ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW)
                 (account-undo-code-update                    ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_UNDOING_ROW      ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW)
                 (account-undo-deployment-status-update       ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_UNDOING_ROW      ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_ROW)
                 (account-same-marked-for-selfdestruct        ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_UNDOING_ROW)
                 (revert-dom-sub-stamps                       ROW_OFFSET_FOR_SELFDESTRUCT_SECOND_ACCOUNT_UNDOING_ROW 2)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                                              ;;
;;    X.5.6 Undoing rows for scenario/SELFDESTRUCT_WONT_REVERT_NOT_YET_MARKED   ;;
;;                                                                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (selfdestruct-scenario-WONT_REVERT_NOT_YET_MARKED-precondition) (* PEEK_AT_SCENARIO
                                                                          (scenario/SELFDESTRUCT_WONT_REVERT_NOT_YET_MARKED)))

(defconstraint selfdestruct-first-undoing-row-for-WONT_REVERT_NOT_YET_MARKED-scenario (:guard (selfdestruct-scenario-WILL_REVERT-precondition))
               (begin
                 (debug (eq! (shift account/ROM_LEX_FLAG          ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW) 0))
                 (debug (eq! (shift account/TRM_FLAG              ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW) 0))
                 (account-same-address-as                         ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW      ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_ROW)
                 (eq!        (shift account/BALANCE_NEW           ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW) 0)
                 (eq!        (shift account/NONCE_NEW             ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW) 0)
                 (account-same-warmth                             ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW)
                 (eq!        (shift account/CODE_SIZE_NEW         ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW) 0)
                 (eq!        (shift account/CODE_HASH_HI_NEW      ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW) EMPTY_KECCAK_HI)
                 (eq!        (shift account/CODE_HASH_LO_NEW      ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW) EMPTY_KECCAK_LO)
                 (shift      (eq!   account/DEPLOYMENT_NUMBER_NEW (+ 1 account/DEPLOYMENT_NUMBER))                   ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW)
                 (shift      (eq!   account/DEPLOYMENT_STATUS_NEW 0                              )                   ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW)
                 (account-same-marked-for-selfdestruct            ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW)
                 (selfdestruct-dom-sub-stamps                     ROW_OFFSET_FOR_SELFDESTRUCT_FIRST_ACCOUNT_UNDOING_ROW)))
