(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                       ;;;;
;;;;    X.5 SELFDESTRUCT   ;;;;
;;;;                       ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;    X.5.1 Introduction     ;;
;;    X.5.2 Representation   ;;
;;    X.5.3 Scenario         ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    selfdestruct-instruction---setting-scenario-sum ()
                  (if-not-zero PEEK_AT_STACK
                               (if-not-zero stack/HALT_FLAG
                                            (if-not-zero (halting-instruction---is-SELFDESTRUCT)
                                                         (if-not-zero (- 1 stack/SUX stack/SOX)
                                                                      (begin
                                                                        (will-eq! PEEK_AT_SCENARIO                            1)
                                                                        (will-eq! (scenario-shorthand---SELFDESTRUCT---sum)   1)))))))

(defconstraint    selfdestruct-instruction---scenario-back-propagation (:guard (selfdestruct-instruction---scenario-precondition))
                  (begin
                    (eq!    (shift  PEEK_AT_STACK                            ROFF_SELFDESTRUCT___STACK_ROW)    1)
                    (eq!    (shift  stack/HALT_FLAG                          ROFF_SELFDESTRUCT___STACK_ROW)    1)
                    (eq!    (shift  (halting-instruction---is-SELFDESTRUCT)  ROFF_SELFDESTRUCT___STACK_ROW)    1)
                    (eq!    (shift  (- 1 stack/SUX stack/SOX)                ROFF_SELFDESTRUCT___STACK_ROW)    1)))
