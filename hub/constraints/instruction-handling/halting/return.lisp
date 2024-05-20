(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                 ;;;;
;;;;    X.Y RETURN   ;;;;
;;;;                 ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;    X.Y.1 Introduction          ;;
;;    X.Y.2 Scenario row seting   ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun  (return-instruction---standard-stack-hypothesis)  (*  PEEK_AT_STACK
                                                              stack/HALT_FLAG
                                                              [ stack/DEC_FLAG 1 ]
                                                              (-  1  stack/SUX )
                                                              )
  )

(defun  (return-instruction---standard-scenario-row)  (* PEEK_AT_SCENARIO
                                                (scenario-shorthand-RETURN-sum)))

(defconstraint   return-instruction---imposing-some-RETURN-scenario    (:guard  (return-instruction---standard-stack-hypothesis))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin  (eq!  (next  PEEK_AT_SCENARIO)             1)
                         (eq!  (next  (scenario-shorthand-RETURN-sum))  1)
                         )
                 )


;; Note: we could pack into a single constraint the last 3 constraints.
(defconstraint   return-instruction---imposing-the-converse    (:guard  (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin  (eq!        (prev  PEEK_AT_STACK)         1)
                         (eq!        (prev  stack/HALT_FLAG)       1)
                         (eq!        (prev  [ stack/DEC_FLAG 1 ])  1)
                         (vanishes!  (prev  (+ stack/SUX stack/SOX)))
                         )
                 )

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    X.Y.3 Shorthands   ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;




(defconst
  RETURN_INSTRUCTION_STACK_ROW_OFFSET                                          -1
  RETURN_INSTRUCTION_SCENARIO_ROW_OFFSET                                        0
  RETURN_INSTRUCTION_CURRENT_CONTEXT_ROW_OFFSET                                 1
  RETURN_INSTRUCTION_FIRST_MISC_ROW_OFFSET                                      2
  RETURN_INSTRUCTION_SECOND_MISC_ROW_OFFSET_DEPLOY_AND_HASH                     3
  RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET                  3
  RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET                 4
  RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET               4
  RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET              5
  RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_EXCEPTION                        3
  RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_MESSAGE_CALL                     3
  RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_EMPTY_DEPLOYMENT_WILL_REVERT     5
  RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_EMPTY_DEPLOYMENT_WONT_REVERT     4
  RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_NONEMPTY_DEPLOYMENT_WILL_REVERT  6
  RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_NONEMPTY_DEPLOYMENT_WONT_REVERT  5
  )



(defun (return-instruction---instruction)                                (shift   stack/INSTRUCTION                        RETURN_INSTRUCTION_STACK_ROW_OFFSET))
(defun (return-instruction---exception-flag-MXPX)                        (shift   stack/MXPX                               RETURN_INSTRUCTION_STACK_ROW_OFFSET))
(defun (return-instruction---exception-flag-OOGX)                        (shift   stack/OOGX                               RETURN_INSTRUCTION_STACK_ROW_OFFSET))
(defun (return-instruction---exception-flag-MAXCSX)                      (shift   stack/MAXCSX                             RETURN_INSTRUCTION_STACK_ROW_OFFSET))
(defun (return-instruction---exception-flag-ICPX)                        (shift   stack/ICPX                               RETURN_INSTRUCTION_STACK_ROW_OFFSET))
(defun (return-instruction---offset-hi)                                  (shift   [ stack/STACK_ITEM_VALUE_HI 1]           RETURN_INSTRUCTION_STACK_ROW_OFFSET))
(defun (return-instruction---offset-lo)                                  (shift   [ stack/STACK_ITEM_VALUE_LO 1]           RETURN_INSTRUCTION_STACK_ROW_OFFSET))
(defun (return-instruction---size-hi)                                    (shift   [ stack/STACK_ITEM_VALUE_HI 2]           RETURN_INSTRUCTION_STACK_ROW_OFFSET))
(defun (return-instruction---size-lo)                                    (shift   [ stack/STACK_ITEM_VALUE_LO 2]           RETURN_INSTRUCTION_STACK_ROW_OFFSET))
(defun (return-instruction---code-hash-hi)                               (shift   stack/HASH_INFO_KECCAK_HI                RETURN_INSTRUCTION_STACK_ROW_OFFSET))
(defun (return-instruction---code-hash-lo)                               (shift   stack/HASH_INFO_KECCAK_LO                RETURN_INSTRUCTION_STACK_ROW_OFFSET))
(defun (return-instruction---is-root)                                    (shift   context/IS_ROOT                          RETURN_INSTRUCTION_CURRENT_CONTEXT_ROW_OFFSET))
(defun (return-instruction---deployment-address-hi)                      (shift   context/BYTE_CODE_ADDRESS_HI             RETURN_INSTRUCTION_CURRENT_CONTEXT_ROW_OFFSET))
(defun (return-instruction---deployment-address-lo)                      (shift   context/BYTE_CODE_ADDRESS_LO             RETURN_INSTRUCTION_CURRENT_CONTEXT_ROW_OFFSET))
(defun (return-instruction---is-deployment)                              (shift   context/BYTE_CODE_DEPLOYMENT_STATUS      RETURN_INSTRUCTION_CURRENT_CONTEXT_ROW_OFFSET))
(defun (return-instruction---return-at-offset)                           (shift   context/RETURN_AT_OFFSET                 RETURN_INSTRUCTION_CURRENT_CONTEXT_ROW_OFFSET))
(defun (return-instruction---return-at-capacity)                         (shift   context/RETURN_AT_CAPACITY               RETURN_INSTRUCTION_CURRENT_CONTEXT_ROW_OFFSET))
(defun (return-instruction---MXP-may-trigger-non-trivial-operation)      (shift   misc/MXP_MTNTOP                          RETURN_INSTRUCTION_FIRST_MISC_ROW_OFFSET))
(defun (return-instruction---MXP-memory-expansion-gas)                   (shift   misc/MXP_GAS_MXP                         RETURN_INSTRUCTION_FIRST_MISC_ROW_OFFSET))
(defun (return-instruction---MXP-memory-expansion-exception)             (shift   misc/MXP_MXPX                            RETURN_INSTRUCTION_FIRST_MISC_ROW_OFFSET))
(defun (return-instruction---MMU-success-bit)                            (shift   misc/MMU_SUCCESS_BIT                     RETURN_INSTRUCTION_FIRST_MISC_ROW_OFFSET))
(defun (return-instruction---OOB-max-code-size-exception)                (shift   [ misc/OOB_DATA 7 ]                      RETURN_INSTRUCTION_FIRST_MISC_ROW_OFFSET))
(defun (return-instruction---deployment-code-fragment-index)             (shift   account/CODE_FRAGMENT_INDEX              RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;    X.Y.4 Generalities   ;;
;;                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint return-instruction---acceptable-exceptions  (:guard  PEEK_AT_SCENARIO)
               ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
               (begin
                 (if-not-zero  (scenario-shorthand-RETURN-message-call)
                               (eq!  XAHOY
                                     (+  (return-instruction---exception-flag-MXPX)
                                         (return-instruction---exception-flag-OOGX))))
                 (if-not-zero  (scenario-shorthand-RETURN-deployment)
                               (eq!  XAHOY
                                     (+  (return-instruction---exception-flag-MXPX)
                                         (return-instruction---exception-flag-OOGX)
                                         (return-instruction---exception-flag-MAXCSX)
                                         (return-instruction---exception-flag-ICPX))))
                 )
               )

(defconstraint   return-instruction---setting-stack-pattern               (:guard  (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (prev (stack-pattern-2-0)
                       )
                 )

(defconstraint   return-instruction---setting-NSR               (:guard  (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq!  NSR
                       (+  (* 4 scenario/RETURN_EXCEPTION                                 )
                           (* 4 scenario/RETURN_FROM_MESSAGE_CALL_WILL_TOUCH_RAM          )
                           (* 4 scenario/RETURN_FROM_MESSAGE_CALL_WONT_TOUCH_RAM          )
                           (* 6 scenario/RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WILL_REVERT    )
                           (* 5 scenario/RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WONT_REVERT    )
                           (* 7 scenario/RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WILL_REVERT )
                           (* 6 scenario/RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WONT_REVERT )
                           )
                       )
                 )
                 
(defconstraint   return-instruction---setting-peeking-flags                   (:guard  (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (if-not-zero   scenario/RETURN_EXCEPTION
                                  (eq!  NSR
                                        (+  (shift  PEEK_AT_SCENARIO        RETURN_INSTRUCTION_SCENARIO_ROW_OFFSET)
                                            (shift  PEEK_AT_CONTEXT         RETURN_INSTRUCTION_CURRENT_CONTEXT_ROW_OFFSET)
                                            (shift  PEEK_AT_MISCELLANEOUS   RETURN_INSTRUCTION_FIRST_MISC_ROW_OFFSET)
                                            (shift  PEEK_AT_CONTEXT         RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_EXCEPTION))))
                   (if-not-zero   scenario/RETURN_FROM_MESSAGE_CALL_WILL_TOUCH_RAM
                                  (eq!  NSR
                                        (+  (shift  PEEK_AT_SCENARIO        RETURN_INSTRUCTION_SCENARIO_ROW_OFFSET)
                                            (shift  PEEK_AT_CONTEXT         RETURN_INSTRUCTION_CURRENT_CONTEXT_ROW_OFFSET)
                                            (shift  PEEK_AT_MISCELLANEOUS   RETURN_INSTRUCTION_FIRST_MISC_ROW_OFFSET)
                                            (shift  PEEK_AT_CONTEXT         RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_MESSAGE_CALL))))
                   (if-not-zero   scenario/RETURN_FROM_MESSAGE_CALL_WONT_TOUCH_RAM
                                  (eq!  NSR
                                        (+  (shift  PEEK_AT_SCENARIO        RETURN_INSTRUCTION_SCENARIO_ROW_OFFSET)
                                            (shift  PEEK_AT_CONTEXT         RETURN_INSTRUCTION_CURRENT_CONTEXT_ROW_OFFSET)
                                            (shift  PEEK_AT_MISCELLANEOUS   RETURN_INSTRUCTION_FIRST_MISC_ROW_OFFSET)
                                            (shift  PEEK_AT_CONTEXT         RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_MESSAGE_CALL))))
                   (if-not-zero   scenario/RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WILL_REVERT
                                  (eq!  NSR
                                        (+  (shift  PEEK_AT_SCENARIO        RETURN_INSTRUCTION_SCENARIO_ROW_OFFSET)
                                            (shift  PEEK_AT_CONTEXT         RETURN_INSTRUCTION_CURRENT_CONTEXT_ROW_OFFSET)
                                            (shift  PEEK_AT_MISCELLANEOUS   RETURN_INSTRUCTION_FIRST_MISC_ROW_OFFSET)
                                            (shift  PEEK_AT_ACCOUNT         RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET )
                                            (shift  PEEK_AT_ACCOUNT         RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET)
                                            (shift  PEEK_AT_CONTEXT         RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_EMPTY_DEPLOYMENT_WILL_REVERT))))
                   (if-not-zero   scenario/RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WONT_REVERT
                                  (eq!  NSR
                                        (+  (shift  PEEK_AT_SCENARIO        RETURN_INSTRUCTION_SCENARIO_ROW_OFFSET)
                                            (shift  PEEK_AT_CONTEXT         RETURN_INSTRUCTION_CURRENT_CONTEXT_ROW_OFFSET)
                                            (shift  PEEK_AT_MISCELLANEOUS   RETURN_INSTRUCTION_FIRST_MISC_ROW_OFFSET)
                                            (shift  PEEK_AT_ACCOUNT         RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET )
                                            (shift  PEEK_AT_CONTEXT         RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_EMPTY_DEPLOYMENT_WONT_REVERT))))
                   (if-not-zero   scenario/RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WILL_REVERT
                                  (eq!  NSR
                                        (+  (shift  PEEK_AT_SCENARIO        RETURN_INSTRUCTION_SCENARIO_ROW_OFFSET)
                                            (shift  PEEK_AT_CONTEXT         RETURN_INSTRUCTION_CURRENT_CONTEXT_ROW_OFFSET)
                                            (shift  PEEK_AT_MISCELLANEOUS   RETURN_INSTRUCTION_FIRST_MISC_ROW_OFFSET)
                                            (shift  PEEK_AT_MISCELLANEOUS   RETURN_INSTRUCTION_SECOND_MISC_ROW_OFFSET_DEPLOY_AND_HASH)
                                            (shift  PEEK_AT_ACCOUNT         RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET )
                                            (shift  PEEK_AT_ACCOUNT         RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET)
                                            (shift  PEEK_AT_CONTEXT         RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_NONEMPTY_DEPLOYMENT_WILL_REVERT))))
                   (if-not-zero   scenario/RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WONT_REVERT
                                  (eq!  NSR
                                        (+  (shift  PEEK_AT_SCENARIO        RETURN_INSTRUCTION_SCENARIO_ROW_OFFSET)
                                            (shift  PEEK_AT_CONTEXT         RETURN_INSTRUCTION_CURRENT_CONTEXT_ROW_OFFSET)
                                            (shift  PEEK_AT_MISCELLANEOUS   RETURN_INSTRUCTION_FIRST_MISC_ROW_OFFSET)
                                            (shift  PEEK_AT_MISCELLANEOUS   RETURN_INSTRUCTION_SECOND_MISC_ROW_OFFSET_DEPLOY_AND_HASH)
                                            (shift  PEEK_AT_ACCOUNT         RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET )
                                            (shift  PEEK_AT_CONTEXT         RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_NONEMPTY_DEPLOYMENT_WONT_REVERT))))
                   )
                 )

(defconstraint   return-instruction---first-context-row                   (:guard  (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (read-context-data   RETURN_INSTRUCTION_CURRENT_CONTEXT_ROW_OFFSET
                                      CONTEXT_NUMBER)
                 )
                 
(defconstraint   return-instruction---refining-the-return-scenario        (:guard  (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (eq!  scenario/RETURN_EXCEPTION  XAHOY)
                   (if-not-zero  (scenario-shorthand-RETURN-unexceptional)
                                 (eq!  (scenario-shorthand-RETURN-deployment)  (return-instruction---is-deployment)))
                   (if-not-zero  (scenario-shorthand-RETURN-deployment)
                                 (begin
                                    (eq!  (scenario-shorthand-RETURN-deployment-will-revert)  CONTEXT_WILL_REVERT)
                                    (eq!  (scenario-shorthand-RETURN-nonempty-deployment)     (return-instruction---MXP-may-trigger-non-trivial-operation))))
                   (if-not-zero  (scenario-shorthand-RETURN-message-call)
                                 (if-zero  (return-touch-ram-expression)
                                           ;; touch_ram_expression = 0
                                           (eq!  scenario/RETURN_FROM_MESSAGE_CALL_WONT_TOUCH_RAM  1)
                                           ;; touch_ram_expression ≠ 0
                                           (eq!  scenario/RETURN_FROM_MESSAGE_CALL_WILL_TOUCH_RAM  1)
                                           )
                                 )
                   )
                 )

(defun  (return-touch-ram-expression)  (*  (-  1  (return-instruction---is-root))
                                           (return-instruction---MXP-may-trigger-non-trivial-operation)
                                           (return-instruction---return-at-capacity)
                                           )
  )

(defconstraint return-instruction---setting-the-first-misc-row  (:guard  (return-instruction---standard-scenario-row))
               ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
               (eq!   (weighted-MISC-flag-sum   RETURN_INSTRUCTION_FIRST_MISC_ROW_OFFSET)
                      (+   (*   MISC_WEIGHT_MMU   (return-instruction---trigger_MMU))
                           (*   MISC_WEIGHT_MXP   (return-instruction---trigger_MXP))
                           (*   MISC_WEIGHT_OOB   (return-instruction---trigger_OOB))
                           )
                      )
               )

(defun  (return-instruction---trigger_MXP)                        1)
(defun  (return-instruction---trigger_OOB)                        (+  (return-instruction---exception-flag-MAXCSX)   (scenario-shorthand-RETURN-nonempty-deployment)))
(defun  (return-instruction---trigger_MMU)                        (+  (return-instruction---check-first-byte)        (return-instruction---write-return-data-to-caller-ram)))
(defun  (return-instruction---check-first-byte)                   (+  (return-instruction---exception-flag-ICPX)     (scenario-shorthand-RETURN-nonempty-deployment)))
(defun  (return-instruction---write-return-data-to-caller-ram)    scenario/RETURN_FROM_MESSAGE_CALL_WILL_TOUCH_RAM)
(defun  (return-instruction---trigger_HASHINFO)                   (scenario-shorthand-RETURN-nonempty-deployment))

(defconstraint   return-instruction---setting-trigger_HASHINFO           (:guard   (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq!   (shift   stack/HASH_INFO_FLAG   RETURN_INSTRUCTION_STACK_ROW_OFFSET)
                        (return-instruction---trigger_HASHINFO)
                        )
                 )

(defconstraint   return-instruction---setting-MXP-data              (:guard   (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (set-MXP-instruction-type-4   RETURN_INSTRUCTION_FIRST_MISC_ROW_OFFSET   ;; row offset kappa
                                               (return-instruction---instruction)           ;; instruction
                                               (return-instruction---is-deployment)         ;; bit modifying the behaviour of RETURN pricing
                                               (return-instruction---offset-hi)             ;; offset high
                                               (return-instruction---offset-lo)             ;; offset low
                                               (return-instruction---size-hi)               ;; size high
                                               (return-instruction---size-lo)               ;; size low
                                               )
                 )

(defconstraint   return-instruction---setting-OOB-data              (:guard   (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (shift   misc/OOB_FLAG     RETURN_INSTRUCTION_FIRST_MISC_ROW_OFFSET)
                                (set-OOB-instruction-deployment   RETURN_INSTRUCTION_FIRST_MISC_ROW_OFFSET   ;; offset
                                                                  (return-instruction---size-hi)             ;; code size hi
                                                                  (return-instruction---size-lo)             ;; code size lo
                                                                  )
                                )
                 )
                                                         
                                
(defconstraint   return-instruction---setting-MMU-data-first-call   (:guard   (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (if-not-zero   (return-instruction---check-first-byte)
                                  (set-MMU-inst-invalid-code-prefix    RETURN_INSTRUCTION_FIRST_MISC_ROW_OFFSET       ;; offset
                                                                       CONTEXT_NUMBER                         ;; source ID
                                                                       ;; tgt_id                              ;; target ID
                                                                       ;; aux_id                              ;; auxiliary ID
                                                                       ;; src_offset_hi                       ;; source offset high
                                                                       (return-instruction---offset-lo)             ;; source offset low
                                                                       ;; tgt_offset_lo                       ;; target offset low
                                                                       ;; size                                ;; size
                                                                       ;; ref_offset                          ;; reference offset
                                                                       ;; ref_size                            ;; reference size
                                                                       (return-instruction---exception-flag-ICPX)   ;; success bit
                                                                       ;; limb_1                              ;; limb 1
                                                                       ;; limb_2                              ;; limb 2
                                                                       ;; exo_sum                             ;; weighted exogenous module flag sum
                                                                       ;; phase                               ;; phase
                                                                       ))
                   (if-not-zero   (return-instruction---write-return-data-to-caller-ram)
                                  (set-MMU-inst-ram-to-ram-sans-padding   RETURN_INSTRUCTION_FIRST_MISC_ROW_OFFSET   ;; offset
                                                                          CONTEXT_NUMBER                      ;; source ID
                                                                          CALLER_CONTEXT_NUMBER               ;; target ID
                                                                          ;; aux_id                              ;; auxiliary ID
                                                                          ;; src_offset_hi                       ;; source offset high
                                                                          (return-instruction---offset-lo)             ;; source offset low
                                                                          ;; tgt_offset_lo                       ;; target offset low
                                                                          (return-instruction---size-lo)               ;; size
                                                                          (return-instruction---return-at-offset)      ;; reference offset
                                                                          (return-instruction---return-at-capacity)    ;; reference size
                                                                          ;; success_bit                         ;; success bit
                                                                          ;; limb_1                              ;; limb 1
                                                                          ;; limb_2                              ;; limb 2
                                                                          ;; exo_sum                             ;; weighted exogenous module flag sum
                                                                          ;; phase                               ;; phase
                                                                          )
                                  )
                   )
                 )

(defconstraint   return-instruction---justifying-the-MXPX           (:guard   (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq!   (return-instruction---exception-flag-MXPX)
                        (return-instruction---MXP-memory-expansion-exception)))

(defconstraint   return-instruction---justifying-the-ICPX           (:guard   (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-zero   (force-bool   (return-instruction---check-first-byte))
                            ;; check_first_byte ≡ 0
                            (vanishes!    (return-instruction---exception-flag-ICPX))
                            ;; check_first_byte ≡ 1
                            (eq!          (return-instruction---exception-flag-ICPX)
                                          (return-instruction---MMU-success-bit))))

(defconstraint   return-instruction---justifying-the-MAXCSX         (:guard   (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-zero   (shift   misc/OOB_FLAG   RETURN_INSTRUCTION_FIRST_MISC_ROW_OFFSET)
                            ;; no OOB call
                            (vanishes!   (return-instruction---exception-flag-MAXCSX))
                            ;; OOB was called
                            (eq!         (return-instruction---exception-flag-MAXCSX)
                                         (return-instruction---OOB-max-code-size-exception))))

(defconstraint   return-instruction---setting-the-gas-cost          (:guard   (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-zero   (force-bin   (return-instruction---gas-cost-required))
                                         ;; we don't require the computation of gas cost
                                         (vanishes!   GAS_COST)
                                         ;; we require the computation of gas cost (either OOGX or unexceptional)
                                         (eq!   GAS_COST
                                                (+   stack/STATIC_GAS
                                                     (return-instruction---MXP-memory-expansion-gas)))))

(defun   (return-instruction---gas-cost-required)   (+  (return-instruction---exception-flag-OOGX)
                                               (scenario-shorthand-RETURN-unexceptional)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    X.Y.4  RETURN/EXCEPTION   ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun   (return-instruction---exception-scenario)   (*   PEEK_AT_SCENARIO   scenario/RETURN_EXCEPTION))

;; redundant
(defconstraint   return-instruction---resetting-the-caller-contexts-return-data          (:guard   (return-instruction---exception-scenario))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (execution-provides-empty-return-data   RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_EXCEPTION))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                  ;;
;;    X.Y.4  RETURN/message_call   ;;
;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun   (return-instruction---message-call-scenario)   (*   PEEK_AT_SCENARIO   (scenario-shorthand-RETURN-message-call)))

(defconstraint   return-instruction---setting-the-callers-new-return-data-message-call-case   (:guard (return-instruction---message-call-scenario))
                 (if-not-zero   (force-bin   (return-instruction---is-root))
                                ;; IS_ROOT = 1
                                (read-context-data
                                  RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_MESSAGE_CALL ;; row offset
                                  CONTEXT_NUMBER                                     ;; context number
                                  )
                                ;; IS_ROOT = 0
                                (provide-return-data 
                                  RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_MESSAGE_CALL ;; row offset
                                  CALLER_CONTEXT_NUMBER                              ;; receiver context
                                  CONTEXT_NUMBER                                     ;; provider context
                                  (return-instruction---offset-lo)                     ;; rdo
                                  (return-instruction---size-lo)                       ;; rds
                                  )
                                )
                 )

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                      ;;
;;    X.Y.4  RETURN/empty_deployment   ;;
;;                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun   (return-instruction---empty-deployment-scenario)   (*   PEEK_AT_SCENARIO   (scenario-shorthand-RETURN-empty-deployment)))

(defconstraint   return-instruction---first-account-row-for-empty-deployments   (:guard   (scenario-shorthand-RETURN-empty-deployment))
                 (begin
                   (eq!   (shift   account/ADDRESS_HI                         RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)    (return-instruction---deployment-address-hi))
                   (eq!   (shift   account/ADDRESS_LO                         RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)    (return-instruction---deployment-address-lo))
                   (account-same-balance                                      RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)
                   (account-same-nonce                                        RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)
                   (eq!   (shift   account/CODE_SIZE_NEW                      RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)    (return-instruction---size-lo))
                   (eq!   (shift   account/CODE_HASH_HI_NEW                   RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)    EMPTY_KECCAK_HI)
                   (eq!   (shift   account/CODE_HASH_LO_NEW                   RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)    EMPTY_KECCAK_LO)
                   (account-same-deployment-number                            RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)
                   (eq!   (shift   account/DEPLOYMENT_STATUS_NEW              RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)   0)
                   (account-same-warmth                                       RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)
                   (account-same-marked-for-selfdestruct                      RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)
                   (vanishes!   (shift   account/ROM_LEX_FLAG                 RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET))
                   (vanishes!   (shift   account/TRM_FLAG                     RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET))
                   (standard-dom-sub-stamps                                   RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET    0)
                   ;; debug constraints
                   (debug   (vanishes!   (shift   account/CODE_SIZE_NEW       RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)))
                   (debug   (eq!         (shift   account/CODE_HASH_HI        RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)    EMPTY_KECCAK_HI))
                   (debug   (eq!         (shift   account/CODE_HASH_LO        RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)    EMPTY_KECCAK_LO))
                   (debug   (eq!         (shift   account/DEPLOYMENT_STATUS   RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)    1))
                   (debug   (account-isnt-precompile                          RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET))
                   )
                 )

(defconstraint   return-instruction---second-account-row-for-empty-deployments   (:guard   (scenario-shorthand-RETURN-empty-deployment))
                 (if-not-zero   scenario/RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WILL_REVERT
                                (begin
                                  (account-same-address-as                     RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET    RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)
                                  (account-undo-balance-update                 RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET    RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)
                                  (account-undo-nonce-update                   RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET    RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)
                                  (account-undo-code-update                    RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET    RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)
                                  (account-undo-deployment-status-update       RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET    RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)
                                  (account-undo-warmth-update                  RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET    RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)
                                  (account-same-marked-for-selfdestruct        RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET)
                                  (vanishes!   (shift   account/ROM_LEX_FLAG   RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET))
                                  (vanishes!   (shift   account/TRM_FLAG       RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET))
                                  (revert-dom-sub-stamps                       RETURN_INSTRUCTION_EMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET    1)
                                  )
                                )
                 )

(defconstraint   return-instruction---setting-the-callers-new-return-data-empty-deployments    (:guard   (scenario-shorthand-RETURN-empty-deployment))
                 (begin
                   (if-not-zero   scenario/RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WILL_REVERT
                                  (if-not-zero   (force-bin   (return-instruction---is-root))
                                                 ;; IS_ROOT  ≡  1
                                                 (read-context-data                       RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_EMPTY_DEPLOYMENT_WILL_REVERT    CONTEXT_NUMBER)
                                                 ;; IS_ROOT  ≡  0
                                                 (execution-provides-empty-return-data    RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_EMPTY_DEPLOYMENT_WILL_REVERT)
                                                 )
                                  )
                   (if-not-zero   scenario/RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WONT_REVERT
                                  (if-not-zero   (force-bin   (return-instruction---is-root))
                                                 ;; IS_ROOT  ≡  1
                                                 (read-context-data                       RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_EMPTY_DEPLOYMENT_WONT_REVERT    CONTEXT_NUMBER)
                                                 ;; IS_ROOT  ≡  0
                                                 (execution-provides-empty-return-data    RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_EMPTY_DEPLOYMENT_WONT_REVERT)
                                                 )
                                  )
                   )
                 )


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;;    X.Y.4  RETURN/nonempty_deployment   ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun   (return-instruction---nonempty-deployment-scenario)   (*   PEEK_AT_SCENARIO   (scenario-shorthand-RETURN-nonempty-deployment)))

(defconstraint   return-instruction---setting-the-second-miscellaneous-row-nonempty-deployments   (:guard    (return-instruction---nonempty-deployment-scenario))
                 (eq!   (weighted-MISC-flag-sum    RETURN_INSTRUCTION_SECOND_MISC_ROW_OFFSET_DEPLOY_AND_HASH)    MISC_WEIGHT_MMU))

(defconstraint   return-instruction---setting-the-second-MMU-instruction   (:guard    (return-instruction---nonempty-deployment-scenario))
                 (set-MMU-inst-ram-to-exo-with-padding
                   RETURN_INSTRUCTION_SECOND_MISC_ROW_OFFSET_DEPLOY_AND_HASH    ;; offset
                   CONTEXT_NUMBER                                               ;; source ID
                   (return-instruction---deployment-code-fragment-index)        ;; target ID
                   (+   1   HUB_STAMP)                                          ;; auxiliary ID
                   ;; src_offset_hi                                             ;; source offset high
                   (return-instruction---offset-lo)                             ;; source offset low
                   ;; tgt_offset_lo                                             ;; target offset low
                   (return-instruction---size-lo)                               ;; size
                   ;; ref_offset                                                ;; reference offset
                   (return-instruction---size-lo)                               ;; reference size
                   0                                                            ;; success bit     <==  here: irrelevant
                   ;; limb_1                                                    ;; limb 1
                   ;; limb_2                                                    ;; limb 2
                   (+   EXO_SUM_WEIGHT_ROM   EXO_SUM_WEIGHT_KEC)                ;; weighted exogenous module flag sum
                   0                                                            ;; phase           <==  here: irrelevant
                   )
                 )

(defconstraint   return-instruction---first-account-row-for-nonempty-deployments   (:guard   (scenario-shorthand-RETURN-nonempty-deployment))
                 (begin
                   (eq!   (shift   account/ADDRESS_HI                         RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)    (return-instruction---deployment-address-hi))
                   (eq!   (shift   account/ADDRESS_LO                         RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)    (return-instruction---deployment-address-lo))
                   (account-same-balance                                      RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)
                   (account-same-nonce                                        RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)
                   (eq!   (shift   account/CODE_SIZE_NEW                      RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)    (return-instruction---size-lo))
                   (eq!   (shift   account/CODE_HASH_HI_NEW                   RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)    (return-instruction---code-hash-hi))
                   (eq!   (shift   account/CODE_HASH_LO_NEW                   RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)    (return-instruction---code-hash-lo))
                   (account-same-deployment-number                            RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)
                   (vanishes!   (shift   account/DEPLOYMENT_STATUS_NEW        RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET))
                   (account-same-warmth                                       RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)
                   (account-same-marked-for-selfdestruct                      RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)
                   (eq!         (shift   account/ROM_LEX_FLAG                 RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)   1)
                   (vanishes!   (shift   account/TRM_FLAG                     RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET))
                   (standard-dom-sub-stamps                                   RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET    0)
                   ;; debug constraints
                   (debug   (eq!         (shift   account/CODE_HASH_HI        RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)    EMPTY_KECCAK_HI))
                   (debug   (eq!         (shift   account/CODE_HASH_LO        RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)    EMPTY_KECCAK_LO))
                   (debug   (eq!         (shift   account/DEPLOYMENT_STATUS   RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)    1))
                   (debug   (account-isnt-precompile                          RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET))
                   )
                 )

(defconstraint   return-instruction---second-account-row-for-nonempty-deployments   (:guard   (scenario-shorthand-RETURN-nonempty-deployment))
                 (if-not-zero   scenario/RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WILL_REVERT
                                (begin
                                  (account-same-address-as                     RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET    RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)
                                  (account-undo-balance-update                 RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET    RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)
                                  (account-undo-nonce-update                   RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET    RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)
                                  (account-undo-code-update                    RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET    RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)
                                  (account-undo-deployment-status-update       RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET    RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)
                                  (account-undo-warmth-update                  RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET    RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_FIRST_ACCOUNT_ROW_OFFSET)
                                  (account-same-marked-for-selfdestruct        RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET)
                                  (vanishes!   (shift   account/ROM_LEX_FLAG   RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET))
                                  (vanishes!   (shift   account/TRM_FLAG       RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET))
                                  (revert-dom-sub-stamps                       RETURN_INSTRUCTION_NONEMPTY_DEPLOYMENT_SECOND_ACCOUNT_ROW_OFFSET    1)
                                  )
                                )
                 )

(defconstraint   return-instruction---setting-the-callers-new-return-data-nonempty-deployments    (:guard   (scenario-shorthand-RETURN-nonempty-deployment))
                 (begin
                   (if-not-zero   scenario/RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WILL_REVERT
                                  (if-not-zero   (force-bin   (return-instruction---is-root))
                                                 ;; IS_ROOT  ≡  1
                                                 (read-context-data                       RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_NONEMPTY_DEPLOYMENT_WILL_REVERT    CONTEXT_NUMBER)
                                                 ;; IS_ROOT  ≡  0
                                                 (execution-provides-empty-return-data    RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_NONEMPTY_DEPLOYMENT_WILL_REVERT)
                                                 )
                                  )
                   (if-not-zero   scenario/RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WONT_REVERT
                                  (if-not-zero   (force-bin   (return-instruction---is-root))
                                                 ;; IS_ROOT  ≡  1
                                                 (read-context-data                       RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_NONEMPTY_DEPLOYMENT_WONT_REVERT    CONTEXT_NUMBER)
                                                 ;; IS_ROOT  ≡  0
                                                 (execution-provides-empty-return-data    RETURN_INSTRUCTION_CALLER_CONTEXT_ROW_OFFSET_NONEMPTY_DEPLOYMENT_WONT_REVERT)
                                                 )
                                  )
                   )
                 )
