(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                             ;;
;;    X.Y.10 Partial non stack rows for precompile scenarios   ;;
;;                                                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (call-instruction---precompile-entry)    (*    PEEK_AT_SCENARIO    (scenario-shorthand---CALL---precompile)))

(defun    (call-instruction---NSR-first-half)      (+    (*    (+    CALL___first_half_nsr___prc_failure                1)    scenario/CALL_PRC_FAILURE            )
                                                         (*    (+    CALL___first_half_nsr___prc_success_will_revert    1)    scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT)
                                                         (*    (+    CALL___first_half_nsr___prc_success_wont_revert    1)    scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT)))

(defconstraint    call-instruction---setting-partial-non-stack-rows-for-precompiles    (:guard    (call-instruction---precompile-entry))
                  (eq!    (call-instruction---NSR-first-half)
                          (+    (*    (call-instruction---flag-sum-prc-failure-first-half)                scenario/CALL_PRC_FAILURE            )
                                (*    (call-instruction---flag-sum-prc-success-will-revert-first-half)    scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT)
                                (*    (call-instruction---flag-sum-prc-success-wont-revert-first-half)    scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT))
                          ))

(defconstraint    call-instruction---propagating-the-current-HUB_STAMP    (:guard    (call-instruction---precompile-entry))
                  (vanishes!
                    (+    (*    (-    (shift    HUB_STAMP    CALL___first_half_nsr___prc_failure)                (shift    HUB_STAMP    CALL_1st_stack_row___row_offset))    scenario/CALL_PRC_FAILURE            )
                          (*    (-    (shift    HUB_STAMP    CALL___first_half_nsr___prc_success_will_revert)    (shift    HUB_STAMP    CALL_1st_stack_row___row_offset))    scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT)
                          (*    (-    (shift    HUB_STAMP    CALL___first_half_nsr___prc_success_wont_revert)    (shift    HUB_STAMP    CALL_1st_stack_row___row_offset))    scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT))
                    ))
