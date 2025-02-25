(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                               ;;
;;    X.Y.Z.1 Graphical representation           ;;
;;    X.Y.Z.2 Universal constraints for CALLs    ;;
;;                                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (call-instruction---standard-precondition)    (*    PEEK_AT_SCENARIO    (scenario-shorthand---CALL---sum)))


(defconstraint    call-instruction---setting-the-stack-pattern             (:guard    (call-instruction---standard-precondition))
                  (shift    (call-stack-pattern    (force-bin    (+    (call-instruction---is-CALL---unshifted)
                                                                       (call-instruction---is-CALLCODE---unshifted))))
                            CALL_1st_stack_row___row_offset))

(defconstraint    call-instruction---setting-the-success-bit               (:guard    (call-instruction---standard-precondition))
                  (begin
                    (vanishes!    (call-instruction---STACK-success-bit-hi))
                    (eq!          (call-instruction---STACK-success-bit-lo)
                                  (scenario-shorthand---CALL---success))
                    ))

(defconstraint    call-instruction---setting-allowable-exceptions          (:guard    (call-instruction---standard-precondition))
                  (begin
                    (if-not-zero    (call-instruction---is-CALL)
                                    (eq!    XAHOY
                                            (+    (call-instruction---STACK-staticx)
                                                  (call-instruction---STACK-mxpx)
                                                  (call-instruction---STACK-oogx))))
                    (if-not-zero    (+    (call-instruction---is-CALLCODE)
                                          (call-instruction---is-DELEGATECALL)
                                          (call-instruction---is-STATICCALL))
                                    (eq!    XAHOY
                                            (+    (call-instruction---STACK-mxpx)
                                                  (call-instruction---STACK-oogx))))
                    ))

(defconstraint    call-instruction---the-first-context-row                 (:guard    (call-instruction---standard-precondition))
                  (read-context-data    CALL_1st_context_row___row_offset
                                        CONTEXT_NUMBER))

(defconstraint    call-instruction---setting-miscellaneous-flags           (:guard    (call-instruction---standard-precondition))
                  (eq!    (weighted-MISC-flag-sum    CALL_misc_row___row_offset)
                          (+    (*    MISC_WEIGHT_MXP    (call-instruction---trigger_MXP))
                                (*    MISC_WEIGHT_OOB    (call-instruction---trigger_OOB))
                                (*    MISC_WEIGHT_STP    (call-instruction---trigger_STP)))
                          ))

(defconstraint    call-instruction---setting-OOB-instruction-parameters    (:guard    (call-instruction---standard-precondition))
                  (if-not-zero    (shift    misc/OOB_FLAG    CALL_misc_row___row_offset)
                                  (if-not-zero    scenario/CALL_EXCEPTION
                                                  (set-OOB-instruction---xcall    CALL_misc_row___row_offset                     ;; offset
                                                                                  (call-instruction---STACK-value-hi)            ;; value (high part)
                                                                                  (call-instruction---STACK-value-lo)            ;; value (low  part, stack argument of CALL-type instruction)
                                                                                  ))
                                  (if-not-zero    (scenario-shorthand---CALL---unexceptional)
                                                  (set-OOB-instruction---call    CALL_misc_row___row_offset                     ;; offset
                                                                                 (call-instruction---STACK-value-hi)            ;; value   (high part)
                                                                                 (call-instruction---STACK-value-lo)            ;; value   (low  part, stack argument of CALL-type instruction)
                                                                                 (call-instruction---caller-balance)            ;; balance (from caller account)
                                                                                 (call-instruction---current-call-stack-depth)  ;; call stack depth
                                                                                 ))
                                  ))

(defconstraint    call-instruction---justifying-staticx                    (:guard    (call-instruction---standard-precondition))
                  (eq!    (call-instruction---STACK-staticx)
                          (*    (call-instruction---is-CALL)
                                (call-instruction---OOB-nonzero-value)
                                (call-instruction---current-context-is-static))
                          ))

(defconstraint    call-instruction---setting-MXP-instruction-parameters    (:guard    (call-instruction---standard-precondition))
                  (if-not-zero    (shift    misc/MXP_FLAG    CALL_misc_row___row_offset)
                                  (set-MXP-instruction-type-5    CALL_misc_row___row_offset                ;; row offset kappa
                                                                 (call-instruction---STACK-instruction)    ;; instruction
                                                                 (call-instruction---STACK-cdo-hi)         ;; call data offset high
                                                                 (call-instruction---STACK-cdo-lo)         ;; call data offset low
                                                                 (call-instruction---STACK-cds-hi)         ;; call data size high
                                                                 (call-instruction---STACK-cds-lo)         ;; call data size low
                                                                 (call-instruction---STACK-r@o-hi)         ;; return at offset high
                                                                 (call-instruction---STACK-r@o-lo)         ;; return at offset low
                                                                 (call-instruction---STACK-r@c-hi)         ;; return at capacity high
                                                                 (call-instruction---STACK-r@c-lo)         ;; return at capacity low
                                                                 )
                                  ))

(defconstraint    call-instruction---justifying-mxpx                       (:guard    (call-instruction---standard-precondition))
                  (if-not-zero    (shift    misc/MXP_FLAG    CALL_misc_row___row_offset)
                                  (eq!    (call-instruction---STACK-mxpx)
                                          (call-instruction---MXP-memory-expansion-exception))
                                  ))

(defconstraint    call-instruction---setting-STP-instruction-parameters    (:guard    (call-instruction---standard-precondition))
                  (if-not-zero    (shift    misc/STP_FLAG     CALL_misc_row___row_offset)
                                  (set-STP-instruction-call   CALL_misc_row___row_offset                                           ;; relative row offset
                                                              (call-instruction---STACK-instruction)                               ;; instruction
                                                              (call-instruction---STACK-gas-hi)                                    ;; max gas allowance argument, high part
                                                              (call-instruction---STACK-gas-lo)                                    ;; max gas allowance argument, low  part
                                                              (call-instruction---STACK-value-hi)                                  ;; value to transfer, high part
                                                              (call-instruction---STACK-value-lo)                                  ;; value to transfer, low  part
                                                              (* (call-instruction---callee-exists) (call-instruction---is-CALL))  ;; bit indicating target account existence
                                                              (call-instruction---callee-warmth)                                   ;; bit indicating target account warmth
                                                              (call-instruction---MXP-memory-expansion-gas)                        ;; memory expansion gas
                                                              )
                                  ))

(defconstraint    call-instruction---justifying-oogx                       (:guard    (call-instruction---standard-precondition))
                  (if-not-zero    (shift    misc/STP_FLAG     CALL_misc_row___row_offset)
                                  (eq!    (call-instruction---STACK-oogx)    (call-instruction---STP-out-of-gas-exception))
                                  ))

(defconstraint    call-instruction---setting-the-CALL-scenario-flag        (:guard    (call-instruction---standard-precondition))
                  (begin
                    (eq!             scenario/CALL_EXCEPTION                                  XAHOY)
                    (if-not-zero    (scenario-shorthand---CALL---unexceptional)
                                    (eq!    (scenario-shorthand---CALL---abort)
                                            (call-instruction---OOB-aborting-condition)))
                    (if-not-zero    (scenario-shorthand---CALL---abort)
                                    (begin
                                      (eq!              scenario/CALL_ABORT_WILL_REVERT               (call-instruction---caller-will-revert))
                                      (debug    (eq!    scenario/CALL_ABORT_WONT_REVERT    (-    1    (call-instruction---caller-will-revert))))))
                    (if-not-zero    (scenario-shorthand---CALL---entry)
                                    (begin
                                      (eq!    (scenario-shorthand---CALL---precompile)                  (call-instruction---callee-is-precompile))
                                      (eq!    (scenario-shorthand---CALL---externally-owned-account)    (*   (-    1    (call-instruction---callee-is-precompile))
                                                                                                             (-    1    (call-instruction---callee-has-code))))
                                      (eq!    (scenario-shorthand---CALL---smart-contract)              (call-instruction---callee-has-code))))
                    (if-not-zero    (+    scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT    scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT)
                                    (begin
                                      (eq!              scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT               (call-instruction---caller-will-revert))
                                      (debug    (eq!    scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT    (-    1    (call-instruction---caller-will-revert))))))
                    (if-not-zero    (scenario-shorthand---CALL---externally-owned-account)
                                    (begin
                                      (eq!              scenario/CALL_EOA_SUCCESS_CALLER_WILL_REVERT               (call-instruction---caller-will-revert))
                                      (debug    (eq!    scenario/CALL_EOA_SUCCESS_CALLER_WONT_REVERT    (-    1    (call-instruction---caller-will-revert))))))
                    (if-not-zero    (scenario-shorthand---CALL---smart-contract)
                                    (begin
                                      (eq!                (+    scenario/CALL_SMC_FAILURE_CALLER_WILL_REVERT    scenario/CALL_SMC_SUCCESS_CALLER_WILL_REVERT)
                                                          (call-instruction---caller-will-revert))
                                      (debug    (eq!      (+    scenario/CALL_SMC_FAILURE_CALLER_WONT_REVERT    scenario/CALL_SMC_SUCCESS_CALLER_WONT_REVERT)
                                                          (-    1    (call-instruction---caller-will-revert))))
                                      (eq!                (scenario-shorthand---CALL---smc-failure)               (call-instruction---callee-self-reverts))
                                      (eq!                (scenario-shorthand---CALL---smc-success)    (-    1    (call-instruction---callee-self-reverts)))))))

(defconstraint    call-instruction---setting-the-next-context-number       (:guard    (call-instruction---standard-precondition))
                  (begin
                    (if-not-zero    scenario/CALL_EXCEPTION                        (shift    (next-context-is-caller)     CALL_1st_stack_row___row_offset))
                    (if-not-zero    (scenario-shorthand---CALL---no-context-change)    (shift    (next-context-is-current)    CALL_1st_stack_row___row_offset))
                    (if-not-zero    (scenario-shorthand---CALL---smart-contract)       (shift    (next-context-is-new)        CALL_1st_stack_row___row_offset))))

(defconstraint    call-instruction---setting-GAS_COST                      (:guard    (call-instruction---standard-precondition))
                  (begin
                    (if-not-zero    (+    (call-instruction---STACK-staticx)
                                          (call-instruction---STACK-mxpx))    (vanishes!    GAS_COST))
                    (if-not-zero    (+    (call-instruction---STACK-oogx)
                                          (scenario-shorthand---CALL---abort)
                                          (scenario-shorthand---CALL---smart-contract)
                                          (scenario-shorthand---CALL---externally-owned-account)
                                          (scenario-shorthand---CALL---precompile))
                                    (eq!    GAS_COST    (call-instruction---STP-gas-upfront)))))

(defconstraint    call-instruction---setting-GAS_NEXT                      (:guard    (call-instruction---standard-precondition))
                  (begin
                    (if-not-zero    (+    (call-instruction---STACK-staticx)
                                          (call-instruction---STACK-mxpx)
                                          (call-instruction---STACK-oogx))
                                    (vanishes!    GAS_NEXT))
                    (if-not-zero    (+    (scenario-shorthand---CALL---abort)
                                          (scenario-shorthand---CALL---externally-owned-account))
                                    (eq!    GAS_NEXT
                                            (+    (-    GAS_ACTUAL    (call-instruction---STP-gas-upfront))
                                                  (call-instruction---STP-call-stipend))))
                    (if-not-zero    (scenario-shorthand---CALL---smart-contract)
                                    (eq!    GAS_NEXT
                                            (-    GAS_ACTUAL
                                                  (call-instruction---STP-gas-upfront)
                                                  (call-instruction---STP-gas-paid-out-of-pocket))))
                    ;; (if-not-zero    (scenario-shorthand---CALL---precompile)    ( ... ))   ;; can't be done here !!!
                    ))
