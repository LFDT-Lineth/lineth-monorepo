(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                               ;;;;
;;;;    X.5 Instruction handling   ;;;;
;;;;                               ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;    X.5.2 Instructions raising the SUX_FLAG or SOX_FLAG   ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint handling-stack-exceptions (:perspective stack)
               (if-not-zero (+ SUX SOX)
                            (begin
                              (eq! NON_STACK_ROWS 1)
                              (empty-stack-pattern))))
