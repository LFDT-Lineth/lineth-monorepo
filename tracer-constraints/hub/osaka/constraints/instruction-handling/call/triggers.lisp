(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                            ;;
;;    X.Y.7 Module triggers   ;;
;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (call-instruction---trigger_OOB)                           (+    (*    (call-instruction---is-CALL)    scenario/CALL_EXCEPTION)
                                                                           (scenario-shorthand---CALL---unexceptional)))
(defun    (call-instruction---trigger_MXP)                           (+    (call-instruction---STACK-mxpx)
                                                                           (call-instruction---STACK-oogx)
                                                                           (scenario-shorthand---CALL---unexceptional)))
(defun    (call-instruction---trigger_TRM)                           (+    (call-instruction---STACK-oogx)
                                                                           (scenario-shorthand---CALL---unexceptional)))
(defun    (call-instruction---trigger_STP)                           (+    (call-instruction---STACK-oogx)
                                                                           (scenario-shorthand---CALL---unexceptional)))
(defun    (call-instruction---trigger_ROMLEX)                        (+    (scenario-shorthand---CALL---smart-contract)))
(defun    (call-instruction---call-requires-callee-account)          (+    (shift    misc/STP_FLAG    CALL_misc_row___row_offset)))
(defun    (call-instruction---call-requires-caller-account)          (+    (scenario-shorthand---CALL---unexceptional)))
(defun    (call-instruction---call-requires-accounts-twice)          (+    (scenario-shorthand---CALL---requires-both-accounts-twice)))
(defun    (call-instruction---call-requires-accounts-thrice)         (+    (scenario-shorthand---CALL---requires-both-accounts-thrice)))
