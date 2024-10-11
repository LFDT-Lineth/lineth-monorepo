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
;;    X.Y.Z.5 Final context row for CALL's to smart contracts    ;;
;;                                                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    call-instruction---final-context-row-for-unexceptional-unaborted-SMC-CALLs    (:guard    PEEK_AT_SCENARIO)
                  (begin
                    (if-not-zero    scenario/CALL_SMC_FAILURE_CALLER_WILL_REVERT
                                    (begin
                                      (justify-callee-revert-data       CALL_SMC_failure_will_revert_initialize_callee_context_row___row_offset)
                                      (initialize-callee-context        CALL_SMC_failure_will_revert_initialize_callee_context_row___row_offset)
                                      (first-row-of-callee-context      CALL_SMC_failure_will_revert_initialize_callee_context_row___row_offset)
                                      ))
                    (if-not-zero    scenario/CALL_SMC_FAILURE_CALLER_WONT_REVERT
                                    (begin
                                      (justify-callee-revert-data       CALL_SMC_failure_wont_revert_initialize_callee_context_row___row_offset)
                                      (initialize-callee-context        CALL_SMC_failure_wont_revert_initialize_callee_context_row___row_offset)
                                      (first-row-of-callee-context      CALL_SMC_failure_wont_revert_initialize_callee_context_row___row_offset)
                                      ))
                    (if-not-zero    scenario/CALL_SMC_SUCCESS_CALLER_WILL_REVERT
                                    (begin
                                      (justify-callee-revert-data       CALL_SMC_success_will_revert_initialize_callee_context_row___row_offset)
                                      (initialize-callee-context        CALL_SMC_success_will_revert_initialize_callee_context_row___row_offset)
                                      (first-row-of-callee-context      CALL_SMC_success_will_revert_initialize_callee_context_row___row_offset)
                                      ))
                    (if-not-zero    scenario/CALL_SMC_SUCCESS_CALLER_WONT_REVERT
                                    (begin
                                      (justify-callee-revert-data       CALL_SMC_success_wont_revert_initialize_callee_context_row___row_offset)
                                      (initialize-callee-context        CALL_SMC_success_wont_revert_initialize_callee_context_row___row_offset)
                                      (first-row-of-callee-context      CALL_SMC_success_wont_revert_initialize_callee_context_row___row_offset)
                                      ))
                    ))
