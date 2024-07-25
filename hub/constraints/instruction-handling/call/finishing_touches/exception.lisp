(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                        ;;
;;    X.Y.Z.2 Final context row for exceptional CALL's    ;;
;;                                                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    call-instruction---final-context-row-for-exceptional-CALLs    (:guard    (*    PEEK_AT_SCENARIO    scenario/CALL_EXCEPTION))
                  (begin
                    (if-not-zero    (call-instruction---STACK-staticx)
                                    (execution-provides-empty-return-data    CALL_staticx_update_parent_context_row___row_offset))
                    (if-not-zero    (call-instruction---STACK-mxpx)
                                    (execution-provides-empty-return-data    CALL_mxpx_update_parent_context_row___row_offset))
                    (if-not-zero    (call-instruction---STACK-oogx)
                                    (execution-provides-empty-return-data    CALL_oogx_update_parent_context_row___row_offset))
                    ))
