(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (call-instruction---summon-accounts-once-or-more)    (*    PEEK_AT_SCENARIO
                                                                     (scenario-shorthand---CALL---sum)
                                                                     (+  (call-instruction---STACK-oogx)
                                                                         (scenario-shorthand---CALL---unexceptional))
                                                                     ))

(defun    (call-instruction---summon-accounts-twice-or-more)   (*    PEEK_AT_SCENARIO
                                                                     (scenario-shorthand---CALL---requires-both-accounts-twice)
                                                                     ))

(defun    (call-instruction---summon-accounts-thrice)          (*    PEEK_AT_SCENARIO
                                                                     (scenario-shorthand---CALL---requires-both-accounts-thrice)
                                                                     ))

