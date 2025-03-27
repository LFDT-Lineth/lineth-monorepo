(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                        ;;
;;    X.4 Stack consistency constraints   ;;
;;                                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    stack-consistency---only-nontrivial-contexts     (:guard    stkcp_PEEK_AT_STACK_POW_4)
                  (is-not-zero!    stkcp_CN_POW_4))

(defconstraint    stack-consistency---setting-FIRST-AGAIN ()
                  (begin
                    (eq!    (+    stkcp_FIRST_CTXT    stkcp_AGAIN_CTXT)    stkcp_PEEK_AT_STACK_POW_4)
                    (eq!    (+    stkcp_FIRST_SPOT    stkcp_AGAIN_SPOT)    stkcp_PEEK_AT_STACK_POW_4)
                    (if-zero    (force-bin    stkcp_PEEK_AT_STACK_POW_4)
                                (eq!   (next    (+    stkcp_FIRST_CTXT    stkcp_FIRST_SPOT))
                                       (next    (*    stkcp_PEEK_AT_STACK_POW_4    2))))
                    (if-not-zero    stkcp_PEEK_AT_STACK_POW_4
                                    (if-not-zero    (next     stkcp_PEEK_AT_STACK_POW_4)
                                                    (if-not-zero    (will-remain-constant!    stkcp_CN_POW_4)
                                                                    (eq!   (next    (+    stkcp_FIRST_CTXT    stkcp_FIRST_SPOT))    2)
                                                                    (eq!   (next    stkcp_AGAIN_CTXT)    1))))
                    (if-not-zero    (next    stkcp_AGAIN_CTXT)
                                    (if-not-zero    (will-remain-constant!    stkcp_HEIGHT_1234)
                                                    (eq!    (next    stkcp_FIRST_SPOT)    1)
                                                    (eq!    (next    stkcp_AGAIN_SPOT)    1)))))


(defconstraint    stack-consistency---first-and-repeat-encounter-of-context ()
                  (begin
                    (if-not-zero    stkcp_FIRST_CTXT    (vanishes!   stkcp_HEIGHT_1234))
                    (if-not-zero    stkcp_AGAIN_CTXT    (or!        (remained-constant!    stkcp_HEIGHT_1234)
                                                                     (did-inc!    stkcp_HEIGHT_1234   1)))))


(defconstraint    stack-consistency---first-and-repeat-encounter-of-spot ()
                  (begin
                    (if-not-zero    stkcp_FIRST_SPOT    (vanishes!   stkcp_POP_1234))
                    (if-not-zero    stkcp_AGAIN_SPOT
                                    (if-not-zero    stkcp_HEIGHT_1234
                                                    (begin
                                                      (eq!    (+    stkcp_POP_1234    (prev    stkcp_POP_1234))     1)
                                                      (if-not-zero    stkcp_POP_1234
                                                                      (begin
                                                                        (remained-constant!    stkcp_VALUE_HI_1234)
                                                                        (remained-constant!    stkcp_VALUE_LO_1234))))))))
