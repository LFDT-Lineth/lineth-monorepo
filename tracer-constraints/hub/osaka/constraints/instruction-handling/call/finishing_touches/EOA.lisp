(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                                         ;;
;;    X.Y.Z.4 Final context row for CALL's to externally owned accounts    ;;
;;                                                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    call-instruction---final-context-row-for-unexceptional-unaborted-EOA-CALLs    (:guard PEEK_AT_SCENARIO)
                  (begin
                    (if-not-zero    scenario/CALL_EOA_SUCCESS_CALLER_WILL_REVERT
                                    (nonexecution-provides-empty-return-data    CALL_EOA_will_revert_caller_context_row___row_offset))
                    (if-not-zero    scenario/CALL_EOA_SUCCESS_CALLER_WONT_REVERT
                                    (nonexecution-provides-empty-return-data    CALL_EOA_wont_revert_caller_context_row___row_offset))
                    ))
