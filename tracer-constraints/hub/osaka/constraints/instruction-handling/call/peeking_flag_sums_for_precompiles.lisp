(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                               ;;
;;    X.Y.10 Partial peeking flag sums for precompile scenarios  ;;
;;                                                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (call-instruction---flag-sum-prc-failure-first-half)                (+    (call-instruction---extended-call-prefix)
                                                                                    (shift    PEEK_AT_SCENARIO    CALL_2nd_scenario_row_PRC_failure___row_offset)
                                                                                    ))

(defun    (call-instruction---flag-sum-prc-success-will-revert-first-half)    (+    (call-instruction---extended-call-prefix)
                                                                                    (shift    PEEK_AT_ACCOUNT     CALL_2nd_caller_account_row___row_offset)
                                                                                    (shift    PEEK_AT_ACCOUNT     CALL_2nd_callee_account_row___row_offset)
                                                                                    (shift    PEEK_AT_ACCOUNT     CALL_2nd_delegt_account_row___row_offset)
                                                                                    (shift    PEEK_AT_SCENARIO    CALL_2nd_scenario_row_PRC_success_will_revert_2nd_scenario___row_offset)
                                                                                    ))

(defun    (call-instruction---flag-sum-prc-success-wont-revert-first-half)    (+    (call-instruction---extended-call-prefix)
                                                                                    (shift    PEEK_AT_SCENARIO    CALL_2nd_scenario_row_PRC_success_wont_revert_2nd_scenario___row_offset)
                                                                                    ))
