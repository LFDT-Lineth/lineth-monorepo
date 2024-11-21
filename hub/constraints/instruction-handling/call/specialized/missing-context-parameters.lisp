(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                       ;;
;;    X.Y.Z.5 Defining the missing context parameters    ;;
;;                                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (call-instruction---new-context-is-static)            (+    (*    (call-instruction---is-CALL)            (call-instruction---current-context-is-static))
                                                                      (*    (call-instruction---is-CALLCODE)        (call-instruction---current-context-is-static))
                                                                      (*    (call-instruction---is-DELEGATECALL)    (call-instruction---current-context-is-static))
                                                                      (*    (call-instruction---is-STATICCALL)      1)))

(defun    (call-instruction---new-account-address-hi)           (+    (*    (call-instruction---is-CALL)            (call-instruction---callee-address-hi))
                                                                      (*    (call-instruction---is-CALLCODE)        (call-instruction---current-address-hi))
                                                                      (*    (call-instruction---is-DELEGATECALL)    (call-instruction---current-address-hi))
                                                                      (*    (call-instruction---is-STATICCALL)      (call-instruction---callee-address-hi))))

(defun    (call-instruction---new-account-address-lo)           (+    (*    (call-instruction---is-CALL)            (call-instruction---callee-address-lo))
                                                                      (*    (call-instruction---is-CALLCODE)        (call-instruction---current-address-lo))
                                                                      (*    (call-instruction---is-DELEGATECALL)    (call-instruction---current-address-lo))
                                                                      (*    (call-instruction---is-STATICCALL)      (call-instruction---callee-address-lo))))

(defun    (call-instruction---new-account-deployment-number)    (+    (*    (call-instruction---is-CALL)            (shift    account/DEPLOYMENT_NUMBER    CALL_1st_callee_account_row___row_offset))
                                                                      (*    (call-instruction---is-CALLCODE)        (shift    account/DEPLOYMENT_NUMBER    CALL_1st_caller_account_row___row_offset))
                                                                      (*    (call-instruction---is-DELEGATECALL)    (shift    account/DEPLOYMENT_NUMBER    CALL_1st_caller_account_row___row_offset))
                                                                      (*    (call-instruction---is-STATICCALL)      (shift    account/DEPLOYMENT_NUMBER    CALL_1st_callee_account_row___row_offset))))

(defun    (call-instruction---new-caller-address-hi)            (+    (*    (call-instruction---is-CALL)            (call-instruction---current-address-hi))
                                                                      (*    (call-instruction---is-CALLCODE)        (call-instruction---current-address-hi))
                                                                      (*    (call-instruction---is-DELEGATECALL)    (call-instruction---current-caller-address-hi))
                                                                      (*    (call-instruction---is-STATICCALL)      (call-instruction---current-address-hi))))

(defun    (call-instruction---new-caller-address-lo)            (+    (*    (call-instruction---is-CALL)            (call-instruction---current-address-lo))
                                                                      (*    (call-instruction---is-CALLCODE)        (call-instruction---current-address-lo))
                                                                      (*    (call-instruction---is-DELEGATECALL)    (call-instruction---current-caller-address-lo))
                                                                      (*    (call-instruction---is-STATICCALL)      (call-instruction---current-address-lo))))

(defun    (call-instruction---new-call-value)                   (+    (*    (call-instruction---is-CALL)            (call-instruction---STACK-value-lo))
                                                                      (*    (call-instruction---is-CALLCODE)        (call-instruction---STACK-value-lo))
                                                                      (*    (call-instruction---is-DELEGATECALL)    (call-instruction---current-call-value))
                                                                      (*    (call-instruction---is-STATICCALL)      0)))
