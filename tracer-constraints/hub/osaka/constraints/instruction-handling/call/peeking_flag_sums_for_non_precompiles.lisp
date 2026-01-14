(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                           ;;
;;    X.Y.8 Peeking flag sums for non precompile scenarios   ;;
;;                                                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; prefixes
(defun    (call-instruction---standard-call-prefix)                  (+    (shift    PEEK_AT_SCENARIO         CALL_1st_scenario_row___row_offset)
                                                                           (shift    PEEK_AT_CONTEXT          CALL_1st_context_row___row_offset)
                                                                           (shift    PEEK_AT_MISCELLANEOUS    CALL_misc_row___row_offset)))

(defun    (call-instruction---extended-call-prefix)                  (+    (call-instruction---standard-call-prefix)
                                                                           (shift    PEEK_AT_ACCOUNT    CALL_1st_caller_account_row___row_offset)
                                                                           (shift    PEEK_AT_ACCOUNT    CALL_1st_callee_account_row___row_offset)))



;; exceptional CALLs
(defun    (call-instruction---flag-sum-staticx)                      (+    (call-instruction---standard-call-prefix)
                                                                           (shift    PEEK_AT_CONTEXT    CALL_staticx_update_parent_context_row___row_offset)))

(defun    (call-instruction---flag-sum-mxpx)                         (+    (call-instruction---standard-call-prefix)
                                                                           (shift    PEEK_AT_CONTEXT    CALL_mxpx_update_parent_context_row___row_offset)))

(defun    (call-instruction---flag-sum-oogx)                         (+    (call-instruction---extended-call-prefix)
                                                                           (shift    PEEK_AT_CONTEXT    CALL_oogx_update_parent_context_row___row_offset)))



;; unexceptional yet aborted CALLs
(defun    (call-instruction---flag-sum-abort-will-revert)            (+    (call-instruction---extended-call-prefix)
                                                                           (shift    PEEK_AT_ACCOUNT    CALL_2nd_callee_account_row___abort_will_revert___row_offset)
                                                                           (shift    PEEK_AT_CONTEXT    CALL_ABORT_WILL_REVERT---current-context-update---row-offset)))

(defun    (call-instruction---flag-sum-abort-wont-revert)            (+    (call-instruction---extended-call-prefix)
                                                                           (shift    PEEK_AT_CONTEXT    CALL_ABORT_WONT_REVERT---current-context-update---row-offset)))


;; entering EOA
(defun    (call-instruction---flag-sum-eoa-will-revert)              (+    (call-instruction---extended-call-prefix)
                                                                           (shift    PEEK_AT_ACCOUNT    CALL_2nd_caller_account_row___row_offset)
                                                                           (shift    PEEK_AT_ACCOUNT    CALL_2nd_callee_account_row___row_offset)
                                                                           (shift    PEEK_AT_CONTEXT    CALL_EOA_will_revert_caller_context_row___row_offset)))

(defun    (call-instruction---flag-sum-eoa-wont-revert)              (+    (call-instruction---extended-call-prefix)
                                                                           (shift    PEEK_AT_CONTEXT    CALL_EOA_wont_revert_caller_context_row___row_offset)))



;; entering SMC
(defun    (call-instruction---flag-sum-smc-failure-will-revert)      (+    (call-instruction---extended-call-prefix)
                                                                           (shift    PEEK_AT_ACCOUNT    CALL_2nd_caller_account_row___row_offset)
                                                                           (shift    PEEK_AT_ACCOUNT    CALL_2nd_callee_account_row___row_offset)
                                                                           (shift    PEEK_AT_ACCOUNT    CALL_3rd_callee_account_row___row_offset)
                                                                           (shift    PEEK_AT_CONTEXT    CALL_SMC_failure_will_revert_initialize_callee_context_row___row_offset)))

(defun    (call-instruction---flag-sum-smc-failure-wont-revert)      (+    (call-instruction---extended-call-prefix)
                                                                           (shift    PEEK_AT_ACCOUNT    CALL_2nd_caller_account_row___row_offset)
                                                                           (shift    PEEK_AT_ACCOUNT    CALL_2nd_callee_account_row___row_offset)
                                                                           (shift    PEEK_AT_CONTEXT    CALL_SMC_failure_wont_revert_initialize_callee_context_row___row_offset)))

(defun    (call-instruction---flag-sum-smc-success-will-revert)      (+    (call-instruction---extended-call-prefix)
                                                                           (shift    PEEK_AT_ACCOUNT    CALL_2nd_caller_account_row___row_offset)
                                                                           (shift    PEEK_AT_ACCOUNT    CALL_2nd_callee_account_row___row_offset)
                                                                           (shift    PEEK_AT_CONTEXT    CALL_SMC_success_will_revert_initialize_callee_context_row___row_offset)))

(defun    (call-instruction---flag-sum-smc-success-wont-revert)      (+    (call-instruction---extended-call-prefix)
                                                                           (shift    PEEK_AT_CONTEXT    CALL_SMC_success_wont_revert_initialize_callee_context_row___row_offset)))
