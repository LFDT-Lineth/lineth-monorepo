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
;;    X.Y.9 Non stack rows for non precompiles   ;;
;;                                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (call-instruction---anything-but-precompile-entry)    (*    PEEK_AT_SCENARIO    (scenario-shorthand---CALL---no-precompile)))

(defconstraint    call-instruction---setting-NSR-for-non-precompiles    (:guard    (call-instruction---anything-but-precompile-entry))
                  (eq!    (shift    NON_STACK_ROWS    CALL_1st_stack_row___row_offset)
                          (+    (*    (+    CALL_nsr___staticx                    1)    (call-instruction---STACK-staticx))
                                (*    (+    CALL_nsr___mxpx                       1)    (call-instruction---STACK-mxpx))
                                (*    (+    CALL_nsr___oogx                       1)    (call-instruction---STACK-oogx))
                                (*    (+    CALL_nsr___abort_will_revert          1)    scenario/CALL_ABORT_WILL_REVERT)
                                (*    (+    CALL_nsr___abort_wont_revert          1)    scenario/CALL_ABORT_WONT_REVERT)
                                (*    (+    CALL_nsr___eoa_success_will_revert    1)    scenario/CALL_EOA_SUCCESS_CALLER_WILL_REVERT)
                                (*    (+    CALL_nsr___eoa_success_wont_revert    1)    scenario/CALL_EOA_SUCCESS_CALLER_WONT_REVERT)
                                (*    (+    CALL_nsr___smc_failure_will_revert    1)    scenario/CALL_SMC_FAILURE_CALLER_WILL_REVERT)
                                (*    (+    CALL_nsr___smc_failure_wont_revert    1)    scenario/CALL_SMC_FAILURE_CALLER_WONT_REVERT)
                                (*    (+    CALL_nsr___smc_success_will_revert    1)    scenario/CALL_SMC_SUCCESS_CALLER_WILL_REVERT)
                                (*    (+    CALL_nsr___smc_success_wont_revert    1)    scenario/CALL_SMC_SUCCESS_CALLER_WONT_REVERT)
                                )))


(defconstraint    call-instruction---setting-flag-sums-for-non-precompiles    (:guard    (call-instruction---anything-but-precompile-entry))
                  (eq!    (shift    NON_STACK_ROWS    CALL_1st_stack_row___row_offset)
                          (+    (*    (call-instruction---flag-sum-staticx)                    (call-instruction---STACK-staticx))
                                (*    (call-instruction---flag-sum-mxpx)                       (call-instruction---STACK-mxpx))
                                (*    (call-instruction---flag-sum-oogx)                       (call-instruction---STACK-oogx))
                                (*    (call-instruction---flag-sum-abort-will-revert)          scenario/CALL_ABORT_WILL_REVERT)
                                (*    (call-instruction---flag-sum-abort-wont-revert)          scenario/CALL_ABORT_WONT_REVERT)
                                (*    (call-instruction---flag-sum-eoa-will-revert)            scenario/CALL_EOA_SUCCESS_CALLER_WILL_REVERT)
                                (*    (call-instruction---flag-sum-eoa-wont-revert)            scenario/CALL_EOA_SUCCESS_CALLER_WONT_REVERT)
                                (*    (call-instruction---flag-sum-smc-failure-will-revert)    scenario/CALL_SMC_FAILURE_CALLER_WILL_REVERT)
                                (*    (call-instruction---flag-sum-smc-failure-wont-revert)    scenario/CALL_SMC_FAILURE_CALLER_WONT_REVERT)
                                (*    (call-instruction---flag-sum-smc-success-will-revert)    scenario/CALL_SMC_SUCCESS_CALLER_WILL_REVERT)
                                (*    (call-instruction---flag-sum-smc-success-wont-revert)    scenario/CALL_SMC_SUCCESS_CALLER_WONT_REVERT)
                                )))
