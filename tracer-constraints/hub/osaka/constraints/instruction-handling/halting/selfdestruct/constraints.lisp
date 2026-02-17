(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                       ;;;;
;;;;    X.5 SELFDESTRUCT   ;;;;
;;;;                       ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                        ;;
;;    X.5.5 Constraints   ;;
;;                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (selfdestruct-instruction---scenario-precondition) (* PEEK_AT_SCENARIO (scenario-shorthand---SELFDESTRUCT---sum)))

(defconstraint    selfdestruct-instruction---looking-back (:guard (selfdestruct-instruction---scenario-precondition))
                  (begin
                    (eq!   (shift   PEEK_AT_STACK                  ROFF_SELFDESTRUCT___STACK_ROW)     1)
                    (eq!   (shift   stack/INSTRUCTION              ROFF_SELFDESTRUCT___STACK_ROW) EVM_INST_SELFDESTRUCT)
                    (eq!   (+   (selfdestruct-instruction---STATICX)    (selfdestruct-instruction---OOGX))
                           XAHOY)))

(defconstraint    selfdestruct-instruction---setting-stack-pattern (:guard (selfdestruct-instruction---scenario-precondition))
                  (shift   (stack-pattern-1-0)   ROFF_SELFDESTRUCT___STACK_ROW))

(defconstraint    selfdestruct-instruction---setting-the-right-scenario (:guard (selfdestruct-instruction---scenario-precondition))
                  (begin
                    (eq! XAHOY scenario/SELFDESTRUCT_EXCEPTION)
                    (if-zero XAHOY
                             (begin
                               (eq! scenario/SELFDESTRUCT_WILL_REVERT             CONTEXT_WILL_REVERT)
                               (eq! (scenario-shorthand---SELFDESTRUCT---wont-revert) (- 1 CONTEXT_WILL_REVERT))))
                    (if-zero CONTEXT_WILL_REVERT
                             (begin
                               (eq! (scenario-shorthand---SELFDESTRUCT---wont-revert)    1)
                               (eq! scenario/SELFDESTRUCT_WONT_REVERT_ALREADY_MARKED (- 1 (selfdestruct-instruction---trigger-future-acc-deletion)))
                               (eq! scenario/SELFDESTRUCT_WONT_REVERT_NOT_YET_MARKED (selfdestruct-instruction---trigger-future-acc-deletion))))))

(defconstraint    selfdestruct-instruction---setting-NSR-and-peeking-flags---STATICX-case
                  (:guard (selfdestruct-instruction---scenario-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero (selfdestruct-instruction---STATICX)
                               (begin
                                 (eq! (shift   NSR   ROFF_SELFDESTRUCT___STACK_ROW) 3)
                                 (eq! (shift   NSR   ROFF_SELFDESTRUCT___STACK_ROW) (+ (shift PEEK_AT_SCENARIO  ROFF_SELFDESTRUCT___SCENARIO_ROW   )
                                                                                       (shift PEEK_AT_CONTEXT   ROFF_SELFDESTRUCT___1ST_CONTEXT_ROW)
                                                                                       (shift PEEK_AT_CONTEXT   ROFF_SELFDESTRUCT___FINAL_CONTEXT_STATICX))))))

(defconstraint    selfdestruct-instruction---setting-NSR-and-peeking-flags---OOGX-case
                  (:guard (selfdestruct-instruction---scenario-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero (selfdestruct-instruction---OOGX)
                               (begin
                                 (eq! (shift   NSR   ROFF_SELFDESTRUCT___STACK_ROW) 5)
                                 (eq! (shift   NSR   ROFF_SELFDESTRUCT___STACK_ROW) (+ (shift PEEK_AT_SCENARIO  ROFF_SELFDESTRUCT___SCENARIO_ROW           )
                                                                                       (shift PEEK_AT_CONTEXT   ROFF_SELFDESTRUCT___1ST_CONTEXT_ROW        )
                                                                                       (shift PEEK_AT_ACCOUNT   ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW)
                                                                                       (shift PEEK_AT_ACCOUNT   ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW)
                                                                                       (shift PEEK_AT_CONTEXT   ROFF_SELFDESTRUCT___FINAL_CONTEXT_OOGX))))))

(defconstraint    selfdestruct-instruction---setting-NSR-and-peeking-flags---WILL_REVERT-case
                  (:guard (selfdestruct-instruction---scenario-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero scenario/SELFDESTRUCT_WILL_REVERT
                               (begin
                                 (eq! (shift   NSR   ROFF_SELFDESTRUCT___STACK_ROW) 7)
                                 (eq! (shift   NSR   ROFF_SELFDESTRUCT___STACK_ROW) (+ (shift PEEK_AT_SCENARIO  ROFF_SELFDESTRUCT___SCENARIO_ROW             )
                                                                                       (shift PEEK_AT_CONTEXT   ROFF_SELFDESTRUCT___1ST_CONTEXT_ROW          )
                                                                                       (shift PEEK_AT_ACCOUNT   ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW  )
                                                                                       (shift PEEK_AT_ACCOUNT   ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW  )
                                                                                       (shift PEEK_AT_ACCOUNT   ROFF_SELFDESTRUCT___ACCOUNT___1ST_UNDOING_ROW)
                                                                                       (shift PEEK_AT_ACCOUNT   ROFF_SELFDESTRUCT___ACCOUNT___2ND_UNDOING_ROW)
                                                                                       (shift PEEK_AT_CONTEXT   ROFF_SELFDESTRUCT___FINAL_CONTEXT_WILL_REVERT))))))

(defconstraint    selfdestruct-instruction---setting-NSR-and-peeking-flags---WONT_REVERT_ALREADY_MARKED-case
                  (:guard (selfdestruct-instruction---scenario-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero scenario/SELFDESTRUCT_WONT_REVERT_ALREADY_MARKED
                               (begin
                                 (eq! (shift   NSR   ROFF_SELFDESTRUCT___STACK_ROW) 5)
                                 (eq! (shift   NSR   ROFF_SELFDESTRUCT___STACK_ROW) (+ (shift PEEK_AT_SCENARIO ROFF_SELFDESTRUCT___SCENARIO_ROW           )
                                                                                       (shift PEEK_AT_CONTEXT  ROFF_SELFDESTRUCT___1ST_CONTEXT_ROW        )
                                                                                       (shift PEEK_AT_ACCOUNT  ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW)
                                                                                       (shift PEEK_AT_ACCOUNT  ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW)
                                                                                       (shift PEEK_AT_CONTEXT  ROFF_SELFDESTRUCT___FINAL_CONTEXT_WONT_REVERT_ALREADY_MARKED))))))

(defconstraint    selfdestruct-instruction---setting-NSR-and-peeking-flags---WONT_REVERT_NOT_YET_MARKED-case
                  (:guard (selfdestruct-instruction---scenario-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero scenario/SELFDESTRUCT_WONT_REVERT_NOT_YET_MARKED
                               (begin
                                 (eq! (shift   NSR   ROFF_SELFDESTRUCT___STACK_ROW) 6)
                                 (eq! (shift   NSR   ROFF_SELFDESTRUCT___STACK_ROW) (+ (shift PEEK_AT_SCENARIO ROFF_SELFDESTRUCT___SCENARIO_ROW           )
                                                                                       (shift PEEK_AT_CONTEXT  ROFF_SELFDESTRUCT___1ST_CONTEXT_ROW        )
                                                                                       (shift PEEK_AT_ACCOUNT  ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW)
                                                                                       (shift PEEK_AT_ACCOUNT  ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW)
                                                                                       (shift PEEK_AT_ACCOUNT  ROFF_SELFDESTRUCT___ACCOUNT_DELETION_ROW   )
                                                                                       (shift PEEK_AT_CONTEXT  ROFF_SELFDESTRUCT___FINAL_CONTEXT_WONT_REVERT_NOT_YET_MARKED))))))

(defconstraint    selfdestruct-instruction---reading-context-data (:guard (selfdestruct-instruction---scenario-precondition))
                  (read-context-data    ROFF_SELFDESTRUCT___1ST_CONTEXT_ROW       ;; row offset
                                        CONTEXT_NUMBER))                                    ;; context to read

(defconstraint    selfdestruct-instruction---returning-empty-return-data (:guard (selfdestruct-instruction---scenario-precondition))
                  (begin
                    (if-not-zero   (selfdestruct-instruction---STATICX)               (execution-provides-empty-return-data ROFF_SELFDESTRUCT___FINAL_CONTEXT_STATICX))
                    (if-not-zero   (selfdestruct-instruction---OOGX)                  (execution-provides-empty-return-data ROFF_SELFDESTRUCT___FINAL_CONTEXT_OOGX))
                    (if-not-zero   scenario/SELFDESTRUCT_WILL_REVERT                  (execution-provides-empty-return-data ROFF_SELFDESTRUCT___FINAL_CONTEXT_WILL_REVERT))
                    (if-not-zero   scenario/SELFDESTRUCT_WONT_REVERT_ALREADY_MARKED   (execution-provides-empty-return-data ROFF_SELFDESTRUCT___FINAL_CONTEXT_WONT_REVERT_ALREADY_MARKED))
                    (if-not-zero   scenario/SELFDESTRUCT_WONT_REVERT_NOT_YET_MARKED   (execution-provides-empty-return-data ROFF_SELFDESTRUCT___FINAL_CONTEXT_WONT_REVERT_NOT_YET_MARKED))))

(defconstraint    selfdestruct-instruction---justifying-the-static-exception (:guard (selfdestruct-instruction---scenario-precondition))
                  (eq!   (selfdestruct-instruction---STATICX)
                         (selfdestruct-instruction---is-static)))

(defconstraint    selfdestruct-instruction---justifying-the-gas-cost (:guard (selfdestruct-instruction---scenario-precondition))
                  (if-zero (selfdestruct-instruction---STATICX)
                           (if-zero (selfdestruct-instruction---balance)
                                    ;; account has zero balance
                                    (eq! GAS_COST
                                         (+ (shift    stack/STATIC_GAS    ROFF_SELFDESTRUCT___STACK_ROW)
                                            (* (- 1 (selfdestruct-instruction---recipient-warmth)) GAS_CONST_G_COLD_ACCOUNT_ACCESS)))
                                    ;; account has nonzero balance
                                    (eq! GAS_COST
                                         (+ (shift    stack/STATIC_GAS    ROFF_SELFDESTRUCT___STACK_ROW)
                                            (* (- 1 (selfdestruct-instruction---recipient-warmth)) GAS_CONST_G_COLD_ACCOUNT_ACCESS)
                                            (* (- 1 (selfdestruct-instruction---recipient-exists)) GAS_CONST_G_NEW_ACCOUNT        ))))))

(defconstraint    selfdestruct-instruction---first-account-row---generalities
                  (:guard (selfdestruct-instruction---scenario-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (if-zero     (selfdestruct-instruction---STATICX)
                                 (begin
                                   (debug (vanishes! (shift account/ROMLEX_FLAG  ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW)))
                                   (debug (vanishes! (shift account/TRM_FLAG     ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW)))
                                   (eq!   (selfdestruct-instruction---account-address-hi)  (shift account/ADDRESS_HI ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW))
                                   (eq!   (selfdestruct-instruction---account-address-lo)  (shift account/ADDRESS_LO ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW))
                                   ;; balance
                                   (account-same-nonce  ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW)
                                   (account-same-warmth ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW)
                                   ;; code
                                   ;; depoyment
                                   ;; selfdestruct marking
                                   (DOM-SUB-stamps---standard    ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW
                                                                 0)
                                   ))))

(defconstraint    selfdestruct-instruction---first-account-row---setting-code-and-deployment
                  (:guard (selfdestruct-instruction---scenario-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-zero     (selfdestruct-instruction---STATICX)
                               (begin
                                 (if-not-zero (selfdestruct-instruction---OOGX)
                                              ;; OOGX = 1
                                              (begin (account-same-code                             ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW)
                                                     (account-same-deployment-number-and-status     ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW))
                                              ;; OOGX = 0
                                              (if-zero (force-bin (selfdestruct-instruction---is-deployment))
                                                       (begin (account-same-code                             ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW)
                                                              (account-same-deployment-number-and-status     ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW))
                                                       (begin
                                                         (eq!        (shift account/CODE_SIZE_NEW               ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW) 0)
                                                         (eq!        (shift account/CODE_HASH_HI_NEW            ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW) EMPTY_KECCAK_HI)
                                                         (eq!        (shift account/CODE_HASH_LO_NEW            ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW) EMPTY_KECCAK_LO)
                                                         (debug (eq! (shift account/DEPLOYMENT_STATUS           ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW) 1))
                                                         (eq!        (shift account/DEPLOYMENT_STATUS_NEW       ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW) 0)
                                                         (account-same-deployment-number                        ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW)))))))

(defconstraint    selfdestruct-instruction---first-account-row---setting-balance-and-marked-for-SELFDESTRUCT
                  (:guard (selfdestruct-instruction---scenario-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-zero     (selfdestruct-instruction---STATICX)
                               (begin
                                 (if-not-zero (selfdestruct-instruction---OOGX)
                                              (begin
                                                (account-same-balance                      ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW)
                                                (account-same-marked-for-deletion          ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW)))
                                 (if-not-zero (scenario-shorthand---SELFDESTRUCT---unexceptional)   (account-decrement-balance-by              ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW      (selfdestruct-instruction---balance)))
                                 (if-not-zero scenario/SELFDESTRUCT_WILL_REVERT                     (account-same-marked-for-deletion          ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW))
                                 (if-not-zero scenario/SELFDESTRUCT_WONT_REVERT_ALREADY_MARKED      (account-same-marked-for-deletion          ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW))
                                 (if-not-zero scenario/SELFDESTRUCT_WONT_REVERT_NOT_YET_MARKED      (account-mark-account-for-deletion         ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW)))))

(defconstraint    selfdestruct-instruction---generalities-for-the-second-account-row
                  (:guard (selfdestruct-instruction---scenario-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (if-zero     (selfdestruct-instruction---STATICX)
                                 (begin
                                   (debug (eq!  (shift account/ROMLEX_FLAG              ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW) 0 ) )
                                   (eq!         (shift account/TRM_FLAG                 ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW) 1 )
                                   (eq!         (shift account/TRM_RAW_ADDRESS_HI       ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW) (selfdestruct-instruction---raw-recipient-address-hi))
                                   (eq!         (shift account/ADDRESS_LO               ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW) (selfdestruct-instruction---raw-recipient-address-lo))
                                   ;; balance
                                   (account-same-nonce                                  ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW)
                                   ;; warmth
                                   (account-same-code                                   ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW)
                                   (account-same-deployment-number-and-status           ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW)
                                   (account-same-marked-for-deletion                    ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW)
                                   (DOM-SUB-stamps---standard                           ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW 1 )))))

(defconstraint    selfdestruct-instruction---balance-and-warmth-for-second-account-row
                  (:guard (selfdestruct-instruction---scenario-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (if-not-zero (selfdestruct-instruction---OOGX)
                                 (begin
                                   (account-same-balance               ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW)
                                   (account-same-warmth                ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW)
                                   ))
                    (if-not-zero (scenario-shorthand---SELFDESTRUCT---unexceptional)
                                 (begin
                                   (account-turn-on-warmth             ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW)
                                   ;; case HAD_CODE_INITIALLY = 1
                                   (if-not-zero (selfdestruct-instruction---had-code-initially)
                                                (account-increment-balance-by           ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW    (selfdestruct-instruction---balance)))
                                   ;; case HAD_CODE_INITIALLY = 0
                                   (if-not-zero (selfdestruct-instruction---had-no-code-initially)
                                                (if-eq-else (selfdestruct-instruction---account-address) (selfdestruct-instruction---recipient-address)
                                                            ;; self destructing account address = recipient address
                                                            (begin
                                                              (debug (vanishes! (shift   account/BALANCE     ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW)))
                                                              (account-same-balance                 ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW))
                                                            ;; self destructing account address ≠ recipient address
                                                            (account-increment-balance-by           ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW    (selfdestruct-instruction---balance)))
                                                )))))

