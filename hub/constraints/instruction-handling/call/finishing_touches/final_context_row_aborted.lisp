(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                                    ;;
;;    X.Y.Z.3 Final context row for (unexceptional) aborted CALL's    ;;
;;                                                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    call-instruction---final-context-row-for-unexceptional-aborted-CALLs    (:guard    (*    PEEK_AT_SCENARIO    scenario/CALL_ABORT))
                  (nonexecution-provides-empty-return-data    CALL_abort_update_current_context_row___row_offset))
