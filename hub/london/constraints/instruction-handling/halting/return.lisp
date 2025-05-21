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

(defun  (return-instruction---standard-precondition)  (*  PEEK_AT_STACK
                                                          stack/HALT_FLAG
                                                          (halting-instruction---is-RETURN)
                                                          (-  1  stack/SUX )))

(defun  (return-instruction---standard-scenario-row)  (* PEEK_AT_SCENARIO
                                                         (scenario-shorthand---RETURN---sum)))

(defconstraint   return-instruction---imposing-some-RETURN-scenario    (:guard  (return-instruction---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin  (eq!  (next  PEEK_AT_SCENARIO)                     1)
                         (eq!  (next  (scenario-shorthand---RETURN---sum))  1)))


;; Note: we could pack into a single constraint the last 3 constraints.
(defconstraint   return-instruction---imposing-the-converse    (:guard  (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin  (eq!        (shift   PEEK_AT_STACK                       ROFF_RETURN___STACK_ROW)  1)
                         (eq!        (shift   stack/HALT_FLAG                     ROFF_RETURN___STACK_ROW)  1)
                         (eq!        (shift   (halting-instruction---is-RETURN)   ROFF_RETURN___STACK_ROW)  1)
                         (vanishes!  (shift   (+ stack/SUX stack/SOX)             ROFF_RETURN___STACK_ROW))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    X.Y.3 Shorthands   ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; as per usual ROFF ≡ ROW_OFFSET
(defconst
  ROFF_RETURN___STACK_ROW                                        -1
  ROFF_RETURN___SCENARIO_ROW                                      0
  ROFF_RETURN___CURRENT_CONTEXT_ROW                               1
  ROFF_RETURN___1ST_MISC_ROW                                      2
  ROFF_RETURN___2ND_MISC_ROW___DEPLOY_AND_HASH                    3
  ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW                3
  ROFF_RETURN___EMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW                4
  ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW             4
  ROFF_RETURN___NONEMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW             5
  ROFF_RETURN___CALLER_CONTEXT___EXCEPTION                        3
  ROFF_RETURN___CALLER_CONTEXT___MESSAGE_CALL                     3
  ROFF_RETURN___CALLER_CONTEXT___EMPTY_DEPLOYMENT_WILL_REVERT     5
  ROFF_RETURN___CALLER_CONTEXT___EMPTY_DEPLOYMENT_WONT_REVERT     4
  ROFF_RETURN___CALLER_CONTEXT___NONEMPTY_DEPLOYMENT_WILL_REVERT  6
  ROFF_RETURN___CALLER_CONTEXT___NONEMPTY_DEPLOYMENT_WONT_REVERT  5
  )



(defun (return-instruction---instruction)                             (shift   stack/INSTRUCTION                     ROFF_RETURN___STACK_ROW))
(defun (return-instruction---exception-flag-MXPX)                     (shift   stack/MXPX                            ROFF_RETURN___STACK_ROW))
(defun (return-instruction---exception-flag-OOGX)                     (shift   stack/OOGX                            ROFF_RETURN___STACK_ROW))
(defun (return-instruction---exception-flag-MAXCSX)                   (shift   stack/MAXCSX                          ROFF_RETURN___STACK_ROW))
(defun (return-instruction---exception-flag-ICPX)                     (shift   stack/ICPX                            ROFF_RETURN___STACK_ROW))
(defun (return-instruction---offset-hi)                               (shift   [ stack/STACK_ITEM_VALUE_HI 1]        ROFF_RETURN___STACK_ROW))
(defun (return-instruction---offset-lo)                               (shift   [ stack/STACK_ITEM_VALUE_LO 1]        ROFF_RETURN___STACK_ROW))
(defun (return-instruction---size-hi)                                 (shift   [ stack/STACK_ITEM_VALUE_HI 2]        ROFF_RETURN___STACK_ROW))
(defun (return-instruction---size-lo)                                 (shift   [ stack/STACK_ITEM_VALUE_LO 2]        ROFF_RETURN___STACK_ROW)) ;; ""
(defun (return-instruction---code-hash-hi)                            (shift   stack/HASH_INFO_KECCAK_HI             ROFF_RETURN___STACK_ROW))
(defun (return-instruction---code-hash-lo)                            (shift   stack/HASH_INFO_KECCAK_LO             ROFF_RETURN___STACK_ROW))
(defun (return-instruction---is-root)                                 (shift   context/IS_ROOT                       ROFF_RETURN___CURRENT_CONTEXT_ROW))
(defun (return-instruction---deployment-address-hi)                   (shift   context/BYTE_CODE_ADDRESS_HI          ROFF_RETURN___CURRENT_CONTEXT_ROW))
(defun (return-instruction---deployment-address-lo)                   (shift   context/BYTE_CODE_ADDRESS_LO          ROFF_RETURN___CURRENT_CONTEXT_ROW))
(defun (return-instruction---is-deployment)                           (shift   context/BYTE_CODE_DEPLOYMENT_STATUS   ROFF_RETURN___CURRENT_CONTEXT_ROW))
(defun (return-instruction---return-at-offset)                        (shift   context/RETURN_AT_OFFSET              ROFF_RETURN___CURRENT_CONTEXT_ROW))
(defun (return-instruction---return-at-capacity)                      (shift   context/RETURN_AT_CAPACITY            ROFF_RETURN___CURRENT_CONTEXT_ROW))
(defun (return-instruction---MXP-memory-expansion-gas)                (shift   misc/MXP_GAS_MXP                      ROFF_RETURN___1ST_MISC_ROW))
(defun (return-instruction---MXP-memory-expansion-exception)          (shift   misc/MXP_MXPX                         ROFF_RETURN___1ST_MISC_ROW))
(defun (return-instruction---MXP-may-trigger-non-trivial-operation)   (shift   misc/MXP_MTNTOP                       ROFF_RETURN___1ST_MISC_ROW))
(defun (return-instruction---MXP-size-1-is-nonzero-and-no-mxpx)       (shift   misc/MXP_SIZE_1_NONZERO_NO_MXPX       ROFF_RETURN___1ST_MISC_ROW))
(defun (return-instruction---MMU-success-bit)                         (shift   misc/MMU_SUCCESS_BIT                  ROFF_RETURN___1ST_MISC_ROW))
(defun (return-instruction---OOB-max-code-size-exception)             (shift   [ misc/OOB_DATA 7 ]                   ROFF_RETURN___1ST_MISC_ROW)) ;; ""
(defun (return-instruction---deployment-code-fragment-index)          (shift   account/CODE_FRAGMENT_INDEX           ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW))

(defun (return-instruction---type-safe-return-data-offset)            (*       (return-instruction---offset-lo)      (return-instruction---MXP-size-1-is-nonzero-and-no-mxpx)))
(defun (return-instruction---type-safe-return-data-size)              (return-instruction---size-lo))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;    X.Y.4 Generalities   ;;
;;                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint return-instruction---acceptable-exceptions  (:guard  PEEK_AT_SCENARIO)
               ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
               (begin
                 (if-not-zero  (scenario-shorthand---RETURN---message-call)
                               (eq!  XAHOY
                                     (+  (return-instruction---exception-flag-MXPX)
                                         (return-instruction---exception-flag-OOGX))))
                 (if-not-zero  (scenario-shorthand---RETURN---deployment)
                               (eq!  XAHOY
                                     (+  (return-instruction---exception-flag-MXPX)
                                         (return-instruction---exception-flag-OOGX)
                                         (return-instruction---exception-flag-MAXCSX)
                                         (return-instruction---exception-flag-ICPX))))))

(defconstraint   return-instruction---setting-stack-pattern               (:guard  (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (shift    (stack-pattern-2-0)   ROFF_RETURN___STACK_ROW))

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
                           )))

(defconstraint   return-instruction---setting-peeking-flags                   (:guard  (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (if-not-zero   scenario/RETURN_EXCEPTION
                                  ;;;;;;;;;;;;;;;;;;;;;;;;;
                                  (eq!  NSR
                                        (+  (shift  PEEK_AT_SCENARIO        ROFF_RETURN___SCENARIO_ROW)
                                            (shift  PEEK_AT_CONTEXT         ROFF_RETURN___CURRENT_CONTEXT_ROW)
                                            (shift  PEEK_AT_MISCELLANEOUS   ROFF_RETURN___1ST_MISC_ROW)
                                            (shift  PEEK_AT_CONTEXT         ROFF_RETURN___CALLER_CONTEXT___EXCEPTION))))
                   (if-not-zero   scenario/RETURN_FROM_MESSAGE_CALL_WILL_TOUCH_RAM
                                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                                  (eq!  NSR
                                        (+  (shift  PEEK_AT_SCENARIO        ROFF_RETURN___SCENARIO_ROW)
                                            (shift  PEEK_AT_CONTEXT         ROFF_RETURN___CURRENT_CONTEXT_ROW)
                                            (shift  PEEK_AT_MISCELLANEOUS   ROFF_RETURN___1ST_MISC_ROW)
                                            (shift  PEEK_AT_CONTEXT         ROFF_RETURN___CALLER_CONTEXT___MESSAGE_CALL))))
                   (if-not-zero   scenario/RETURN_FROM_MESSAGE_CALL_WONT_TOUCH_RAM
                                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                                  (eq!  NSR
                                        (+  (shift  PEEK_AT_SCENARIO        ROFF_RETURN___SCENARIO_ROW)
                                            (shift  PEEK_AT_CONTEXT         ROFF_RETURN___CURRENT_CONTEXT_ROW)
                                            (shift  PEEK_AT_MISCELLANEOUS   ROFF_RETURN___1ST_MISC_ROW)
                                            (shift  PEEK_AT_CONTEXT         ROFF_RETURN___CALLER_CONTEXT___MESSAGE_CALL))))
                   (if-not-zero   scenario/RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WILL_REVERT
                                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                                  (eq!  NSR
                                        (+  (shift  PEEK_AT_SCENARIO        ROFF_RETURN___SCENARIO_ROW)
                                            (shift  PEEK_AT_CONTEXT         ROFF_RETURN___CURRENT_CONTEXT_ROW)
                                            (shift  PEEK_AT_MISCELLANEOUS   ROFF_RETURN___1ST_MISC_ROW)
                                            (shift  PEEK_AT_ACCOUNT         ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                                            (shift  PEEK_AT_ACCOUNT         ROFF_RETURN___EMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW)
                                            (shift  PEEK_AT_CONTEXT         ROFF_RETURN___CALLER_CONTEXT___EMPTY_DEPLOYMENT_WILL_REVERT))))
                   (if-not-zero   scenario/RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WONT_REVERT
                                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                                  (eq!  NSR
                                        (+  (shift  PEEK_AT_SCENARIO        ROFF_RETURN___SCENARIO_ROW)
                                            (shift  PEEK_AT_CONTEXT         ROFF_RETURN___CURRENT_CONTEXT_ROW)
                                            (shift  PEEK_AT_MISCELLANEOUS   ROFF_RETURN___1ST_MISC_ROW)
                                            (shift  PEEK_AT_ACCOUNT         ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                                            (shift  PEEK_AT_CONTEXT         ROFF_RETURN___CALLER_CONTEXT___EMPTY_DEPLOYMENT_WONT_REVERT))))
                   (if-not-zero   scenario/RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WILL_REVERT
                                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                                  (eq!  NSR
                                        (+  (shift  PEEK_AT_SCENARIO        ROFF_RETURN___SCENARIO_ROW)
                                            (shift  PEEK_AT_CONTEXT         ROFF_RETURN___CURRENT_CONTEXT_ROW)
                                            (shift  PEEK_AT_MISCELLANEOUS   ROFF_RETURN___1ST_MISC_ROW)
                                            (shift  PEEK_AT_MISCELLANEOUS   ROFF_RETURN___2ND_MISC_ROW___DEPLOY_AND_HASH)
                                            (shift  PEEK_AT_ACCOUNT         ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                                            (shift  PEEK_AT_ACCOUNT         ROFF_RETURN___NONEMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW)
                                            (shift  PEEK_AT_CONTEXT         ROFF_RETURN___CALLER_CONTEXT___NONEMPTY_DEPLOYMENT_WILL_REVERT))))
                   (if-not-zero   scenario/RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WONT_REVERT
                                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                                  (eq!  NSR
                                        (+  (shift  PEEK_AT_SCENARIO        ROFF_RETURN___SCENARIO_ROW)
                                            (shift  PEEK_AT_CONTEXT         ROFF_RETURN___CURRENT_CONTEXT_ROW)
                                            (shift  PEEK_AT_MISCELLANEOUS   ROFF_RETURN___1ST_MISC_ROW)
                                            (shift  PEEK_AT_MISCELLANEOUS   ROFF_RETURN___2ND_MISC_ROW___DEPLOY_AND_HASH)
                                            (shift  PEEK_AT_ACCOUNT         ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                                            (shift  PEEK_AT_CONTEXT         ROFF_RETURN___CALLER_CONTEXT___NONEMPTY_DEPLOYMENT_WONT_REVERT))))
                   )
                 )

(defconstraint   return-instruction---first-context-row                   (:guard  (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (read-context-data   ROFF_RETURN___CURRENT_CONTEXT_ROW
                                      CONTEXT_NUMBER)
                 )

(defconstraint   return-instruction---refining-the-return-scenario        (:guard  (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (eq!  scenario/RETURN_EXCEPTION  XAHOY)
                   (if-not-zero  (scenario-shorthand---RETURN---unexceptional)
                                 (eq!  (scenario-shorthand---RETURN---deployment)  (return-instruction---is-deployment)))
                   (if-not-zero  (scenario-shorthand---RETURN---deployment)
                                 (begin
                                    (eq!  (scenario-shorthand---RETURN---deployment-will-revert)  CONTEXT_WILL_REVERT)
                                    (eq!  (scenario-shorthand---RETURN---nonempty-deployment)     (return-instruction---MXP-may-trigger-non-trivial-operation))))
                   (if-not-zero  (scenario-shorthand---RETURN---message-call)
                                 (if-zero  (return-touch-ram-expression)
                                           ;; touch_ram_expression = 0
                                           (eq!  scenario/RETURN_FROM_MESSAGE_CALL_WONT_TOUCH_RAM  1)
                                           ;; touch_ram_expression ≠ 0
                                           (eq!  scenario/RETURN_FROM_MESSAGE_CALL_WILL_TOUCH_RAM  1)
                                           ))))

(defun  (return-touch-ram-expression)  (*  (-  1  (return-instruction---is-root))
                                           (return-instruction---MXP-may-trigger-non-trivial-operation)
                                           (return-instruction---return-at-capacity)
                                           ))

(defconstraint return-instruction---setting-the-first-misc-row  (:guard  (return-instruction---standard-scenario-row))
               ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
               (eq!   (weighted-MISC-flag-sum   ROFF_RETURN___1ST_MISC_ROW)
                      (+   (*   MISC_WEIGHT_MMU   (return-instruction---trigger_MMU))
                           (*   MISC_WEIGHT_MXP   (return-instruction---trigger_MXP))
                           (*   MISC_WEIGHT_OOB   (return-instruction---trigger_OOB))
                           )))

(defun  (return-instruction---trigger_MXP)                        1)
(defun  (return-instruction---trigger_OOB)                        (+  (return-instruction---exception-flag-MAXCSX)   (scenario-shorthand---RETURN---nonempty-deployment)))
(defun  (return-instruction---trigger_MMU)                        (+  (return-instruction---check-first-byte)        (return-instruction---write-return-data-to-caller-ram)))
(defun  (return-instruction---check-first-byte)                   (+  (return-instruction---exception-flag-ICPX)     (scenario-shorthand---RETURN---nonempty-deployment)))
(defun  (return-instruction---write-return-data-to-caller-ram)    scenario/RETURN_FROM_MESSAGE_CALL_WILL_TOUCH_RAM)
(defun  (return-instruction---trigger_HASHINFO)                   (scenario-shorthand---RETURN---nonempty-deployment)) ;; ""

(defconstraint   return-instruction---setting-trigger_HASHINFO           (:guard   (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq!   (shift   stack/HASH_INFO_FLAG   ROFF_RETURN___STACK_ROW)
                        (return-instruction---trigger_HASHINFO)))

(defconstraint   return-instruction---setting-MXP-data              (:guard   (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (shift   misc/MXP_FLAG        ROFF_RETURN___1ST_MISC_ROW)
                                (set-MXP-instruction-type-4   ROFF_RETURN___1ST_MISC_ROW   ;; row offset kappa
                                                              (return-instruction---instruction)           ;; instruction
                                                              (return-instruction---is-deployment)         ;; bit modifying the behaviour of RETURN pricing
                                                              (return-instruction---offset-hi)             ;; offset high
                                                              (return-instruction---offset-lo)             ;; offset low
                                                              (return-instruction---size-hi)               ;; size high
                                                              (return-instruction---size-lo)               ;; size low
                                                              )))

(defconstraint   return-instruction---setting-OOB-data              (:guard   (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (shift   misc/OOB_FLAG              ROFF_RETURN___1ST_MISC_ROW)
                                (set-OOB-instruction---deployment   ROFF_RETURN___1ST_MISC_ROW   ;; offset
                                                                    (return-instruction---size-hi)             ;; code size hi
                                                                    (return-instruction---size-lo)             ;; code size lo
                                                                    )))


(defconstraint   return-instruction---setting-MMU-data-first-call   (:guard   (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (if-not-zero   (return-instruction---check-first-byte)
                                  (set-MMU-instruction---invalid-code-prefix    ROFF_RETURN___1ST_MISC_ROW                        ;; offset
                                                                                CONTEXT_NUMBER                                    ;; source ID
                                                                                ;; tgt_id                                         ;; target ID
                                                                                ;; aux_id                                         ;; auxiliary ID
                                                                                ;; src_offset_hi                                  ;; source offset high
                                                                                (return-instruction---offset-lo)                  ;; source offset low
                                                                                ;; tgt_offset_lo                                  ;; target offset low
                                                                                ;; size                                           ;; size
                                                                                ;; ref_offset                                     ;; reference offset
                                                                                ;; ref_size                                       ;; reference size
                                                                                (return-instruction---exception-flag-ICPX)        ;; success bit; this double negation stuff will be resolved by spec issue #715
                                                                                ;; limb_1                                         ;; limb 1
                                                                                ;; limb_2                                         ;; limb 2
                                                                                ;; exo_sum                                        ;; weighted exogenous module flag sum
                                                                                ;; phase                                          ;; phase
                                                                                ))
                   (if-not-zero   (return-instruction---write-return-data-to-caller-ram)
                                  (set-MMU-instruction---ram-to-ram-sans-padding   ROFF_RETURN___1ST_MISC_ROW                   ;; offset
                                                                                   CONTEXT_NUMBER                               ;; source ID
                                                                                   CALLER_CONTEXT_NUMBER                        ;; target ID
                                                                                   ;; aux_id                                    ;; auxiliary ID
                                                                                   ;; src_offset_hi                             ;; source offset high
                                                                                   (return-instruction---offset-lo)             ;; source offset low
                                                                                   ;; tgt_offset_lo                             ;; target offset low
                                                                                   (return-instruction---size-lo)               ;; size
                                                                                   (return-instruction---return-at-offset)      ;; reference offset
                                                                                   (return-instruction---return-at-capacity)    ;; reference size
                                                                                   ;; success_bit                               ;; success bit
                                                                                   ;; limb_1                                    ;; limb 1
                                                                                   ;; limb_2                                    ;; limb 2
                                                                                   ;; exo_sum                                   ;; weighted exogenous module flag sum
                                                                                   ;; phase                                     ;; phase
                                                                                   ))))

(defconstraint   return-instruction---justifying-the-MXPX
                 (:guard   (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq!   (return-instruction---exception-flag-MXPX)
                        (return-instruction---MXP-memory-expansion-exception)))

(defconstraint   return-instruction---justifying-the-ICPX
                 (:guard   (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-zero   (force-bin    (return-instruction---check-first-byte))
                            ;; check_first_byte ≡ 0
                            (vanishes!    (return-instruction---exception-flag-ICPX))
                            ;; check_first_byte ≡ 1
                            (eq!          (return-instruction---exception-flag-ICPX)
                                          (return-instruction---MMU-success-bit))))

(defconstraint   return-instruction---justifying-the-MAXCSX         (:guard   (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-zero   (shift   misc/OOB_FLAG   ROFF_RETURN___1ST_MISC_ROW)
                            ;; no OOB call
                            (vanishes!   (return-instruction---exception-flag-MAXCSX))
                            ;; OOB was called
                            (eq!         (return-instruction---exception-flag-MAXCSX)
                                         (return-instruction---OOB-max-code-size-exception))))

(defconstraint   return-instruction---setting-the-gas-cost          (:guard   (return-instruction---standard-scenario-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-zero   (force-bin   (return-instruction---gas-cost-required))
                            ;; we don't require the computation of gas cost
                            (vanishes!   GAS_COST)
                            ;; we require the computation of gas cost (either OOGX or unexceptional)
                            (eq!   GAS_COST
                                   (+   (shift    stack/STATIC_GAS    ROFF_RETURN___STACK_ROW)
                                        (return-instruction---MXP-memory-expansion-gas)))))

(defun   (return-instruction---gas-cost-required)   (+  (return-instruction---exception-flag-OOGX)
                                                        (scenario-shorthand---RETURN---unexceptional)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    X.Y.4  RETURN/EXCEPTION   ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun   (return-instruction---exception-scenario)   (*   PEEK_AT_SCENARIO   scenario/RETURN_EXCEPTION))

;; redundant
(defconstraint   return-instruction---resetting-the-caller-contexts-return-data          (:guard   (return-instruction---exception-scenario))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (execution-provides-empty-return-data   ROFF_RETURN___CALLER_CONTEXT___EXCEPTION))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                  ;;
;;    X.Y.4  RETURN/message_call   ;;
;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun   (return-instruction---message-call-scenario)   (*   PEEK_AT_SCENARIO   (scenario-shorthand---RETURN---message-call)))

(defconstraint   return-instruction---setting-the-callers-new-return-data-message-call-case   (:guard (return-instruction---message-call-scenario))
                 (provide-return-data     ROFF_RETURN___CALLER_CONTEXT___MESSAGE_CALL           ;; row offset
                                          CALLER_CONTEXT_NUMBER                                 ;; receiver context
                                          CONTEXT_NUMBER                                        ;; provider context
                                          (return-instruction---type-safe-return-data-offset)   ;; (type safe) rdo
                                          (return-instruction---type-safe-return-data-size)     ;; (type safe) rds
                                          ))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                      ;;
;;    X.Y.4  RETURN/empty_deployment   ;;
;;                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun   (return-instruction---empty-deployment-scenario)   (*   PEEK_AT_SCENARIO   (scenario-shorthand---RETURN---empty-deployment)))

(defconstraint   return-instruction---first-account-row-for-empty-deployments   (:guard   (return-instruction---empty-deployment-scenario))
                 (begin
                   (eq!   (shift   account/ADDRESS_HI                         ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)    (return-instruction---deployment-address-hi))
                   (eq!   (shift   account/ADDRESS_LO                         ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)    (return-instruction---deployment-address-lo))
                   (account-same-balance                                      ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                   (account-same-nonce                                        ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                   (eq!   (shift   account/CODE_SIZE_NEW                      ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)    (return-instruction---size-lo))
                   (eq!   (shift   account/CODE_HASH_HI_NEW                   ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)    EMPTY_KECCAK_HI)
                   (eq!   (shift   account/CODE_HASH_LO_NEW                   ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)    EMPTY_KECCAK_LO)
                   (account-same-deployment-number                            ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                   (eq!   (shift   account/DEPLOYMENT_STATUS_NEW              ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)   0)
                   (account-same-warmth                                       ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                   (account-same-marked-for-selfdestruct                      ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                   (vanishes!   (shift   account/ROMLEX_FLAG                  ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW))
                   (vanishes!   (shift   account/TRM_FLAG                     ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW))
                   (DOM-SUB-stamps---standard                                 ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW    0)
                   ;; debug constraints
                   (debug   (vanishes!   (shift   account/CODE_SIZE_NEW       ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)))
                   (debug   (eq!         (shift   account/CODE_HASH_HI        ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)    EMPTY_KECCAK_HI))
                   (debug   (eq!         (shift   account/CODE_HASH_LO        ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)    EMPTY_KECCAK_LO))
                   (debug   (eq!         (shift   account/DEPLOYMENT_STATUS   ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)    1))
                   (debug   (account-isnt-precompile                          ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW))
                   ))

(defconstraint   return-instruction---second-account-row-for-empty-deployments   (:guard   (return-instruction---empty-deployment-scenario))
                 (if-not-zero   scenario/RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WILL_REVERT
                                (begin
                                  (account-same-address-as                     ROFF_RETURN___EMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW    ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                                  (account-undo-balance-update                 ROFF_RETURN___EMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW    ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                                  (account-undo-nonce-update                   ROFF_RETURN___EMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW    ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                                  (account-undo-code-update                    ROFF_RETURN___EMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW    ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                                  (account-undo-deployment-status-update       ROFF_RETURN___EMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW    ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                                  (account-undo-warmth-update                  ROFF_RETURN___EMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW    ROFF_RETURN___EMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                                  (account-same-marked-for-selfdestruct        ROFF_RETURN___EMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW)
                                  (vanishes!   (shift   account/ROMLEX_FLAG    ROFF_RETURN___EMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW))
                                  (vanishes!   (shift   account/TRM_FLAG       ROFF_RETURN___EMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW))
                                  (DOM-SUB-stamps---revert-with-current        ROFF_RETURN___EMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW    1)
                                  )))

(defconstraint   return-instruction---empty-deployment---squasing-the-creators-return-data    (:guard   (return-instruction---empty-deployment-scenario))
                 (begin
                   (if-not-zero   scenario/RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WILL_REVERT
                                  (execution-provides-empty-return-data    ROFF_RETURN___CALLER_CONTEXT___EMPTY_DEPLOYMENT_WILL_REVERT))
                   (if-not-zero   scenario/RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WONT_REVERT
                                  (execution-provides-empty-return-data    ROFF_RETURN___CALLER_CONTEXT___EMPTY_DEPLOYMENT_WONT_REVERT))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;;    X.Y.4  RETURN/nonempty_deployment   ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun   (return-instruction---nonempty-deployment-scenario)   (*   PEEK_AT_SCENARIO   (scenario-shorthand---RETURN---nonempty-deployment)))

(defconstraint   return-instruction---setting-the-second-miscellaneous-row-nonempty-deployments   (:guard    (return-instruction---nonempty-deployment-scenario))
                 (eq!   (weighted-MISC-flag-sum    ROFF_RETURN___2ND_MISC_ROW___DEPLOY_AND_HASH)    MISC_WEIGHT_MMU))

(defconstraint   return-instruction---setting-the-second-MMU-instruction   (:guard    (return-instruction---nonempty-deployment-scenario))
                 (set-MMU-instruction---ram-to-exo-with-padding    ROFF_RETURN___2ND_MISC_ROW___DEPLOY_AND_HASH    ;; offset
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
                                                                   ))

(defconstraint   return-instruction---first-account-row-for-nonempty-deployments   (:guard   (return-instruction---nonempty-deployment-scenario))
                 (begin
                   (eq!   (shift   account/ADDRESS_HI                         ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)    (return-instruction---deployment-address-hi))
                   (eq!   (shift   account/ADDRESS_LO                         ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)    (return-instruction---deployment-address-lo))
                   (account-same-balance                                      ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                   (account-same-nonce                                        ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                   (eq!   (shift   account/CODE_SIZE_NEW                      ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)    (return-instruction---size-lo))
                   (eq!   (shift   account/CODE_HASH_HI_NEW                   ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)    (return-instruction---code-hash-hi))
                   (eq!   (shift   account/CODE_HASH_LO_NEW                   ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)    (return-instruction---code-hash-lo))
                   (account-same-deployment-number                            ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                   (vanishes!   (shift   account/DEPLOYMENT_STATUS_NEW        ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW))
                   (account-same-warmth                                       ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                   (account-same-marked-for-selfdestruct                      ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                   (eq!         (shift   account/ROMLEX_FLAG                  ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)   1)
                   (vanishes!   (shift   account/TRM_FLAG                     ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW))
                   (DOM-SUB-stamps---standard                                 ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW    0)
                   ;; debug constraints
                   (debug   (eq!         (shift   account/CODE_HASH_HI        ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)    EMPTY_KECCAK_HI))
                   (debug   (eq!         (shift   account/CODE_HASH_LO        ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)    EMPTY_KECCAK_LO))
                   (debug   (eq!         (shift   account/DEPLOYMENT_STATUS   ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)    1))
                   (debug   (account-isnt-precompile                          ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW))
                   ))

(defconstraint   return-instruction---second-account-row-for-nonempty-deployments   (:guard   (return-instruction---nonempty-deployment-scenario))
                 (if-not-zero   scenario/RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WILL_REVERT
                                (begin
                                  (account-same-address-as                     ROFF_RETURN___NONEMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW    ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                                  (account-undo-balance-update                 ROFF_RETURN___NONEMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW    ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                                  (account-undo-nonce-update                   ROFF_RETURN___NONEMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW    ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                                  (account-undo-code-update                    ROFF_RETURN___NONEMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW    ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                                  (account-undo-deployment-status-update       ROFF_RETURN___NONEMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW    ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                                  (account-undo-warmth-update                  ROFF_RETURN___NONEMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW    ROFF_RETURN___NONEMPTY_DEPLOYMENT___1ST_ACCOUNT_ROW)
                                  (account-same-marked-for-selfdestruct        ROFF_RETURN___NONEMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW)
                                  (vanishes!   (shift   account/ROMLEX_FLAG    ROFF_RETURN___NONEMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW))
                                  (vanishes!   (shift   account/TRM_FLAG       ROFF_RETURN___NONEMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW))
                                  (DOM-SUB-stamps---revert-with-current        ROFF_RETURN___NONEMPTY_DEPLOYMENT___2ND_ACCOUNT_ROW    1)
                                  )))

(defconstraint   return-instruction---nonempty-deployment---squasing-the-creators-return-data    (:guard   (return-instruction---nonempty-deployment-scenario))
                 (begin
                   (if-not-zero   scenario/RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WILL_REVERT
                                  (execution-provides-empty-return-data    ROFF_RETURN___CALLER_CONTEXT___NONEMPTY_DEPLOYMENT_WILL_REVERT))
                   (if-not-zero   scenario/RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WONT_REVERT
                                  (execution-provides-empty-return-data    ROFF_RETURN___CALLER_CONTEXT___NONEMPTY_DEPLOYMENT_WONT_REVERT))))
