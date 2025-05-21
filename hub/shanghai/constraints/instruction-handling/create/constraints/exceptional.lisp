(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                 ;;;;
;;;;    X.Y CREATE   ;;;;
;;;;                 ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                  ;;
;;    X.Y.11 Exceptional CREATE's   ;;
;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (create-instruction---exceptional-CREATE-precondition)    (*    PEEK_AT_SCENARIO    scenario/CREATE_EXCEPTION))

(defconstraint    create-instruction---exceptional-CREATE-updating-the-caller-context    (:guard    (create-instruction---exceptional-CREATE-precondition))
                  (execution-provides-empty-return-data    CREATE_exception_caller_context_row___row_offset))
