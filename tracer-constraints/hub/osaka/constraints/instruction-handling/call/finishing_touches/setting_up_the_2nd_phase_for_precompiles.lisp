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


(defconstraint    call-instruction---setting-up-the-second-phase-of-CALLs-to-precompiles    (:guard PEEK_AT_SCENARIO)
                  (begin
                    (if-not-zero    scenario/CALL_PRC_FAILURE
                                    (precompile-scenario-row-setting    CALL_2nd_scenario_row_PRC_failure___row_offset))
                    (if-not-zero    scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT
                                    (precompile-scenario-row-setting    CALL_2nd_scenario_row_PRC_success_will_revert_2nd_scenario___row_offset))
                    (if-not-zero    scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT
                                    (precompile-scenario-row-setting    CALL_2nd_scenario_row_PRC_success_wont_revert_2nd_scenario___row_offset))
                    ))
