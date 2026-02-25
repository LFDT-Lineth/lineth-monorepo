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


(defun    (call-instruction---child-frame---is-static)
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  (+    (*    (call-instruction---is-CALL)            (call-instruction---current-frame---context-is-static ))
        (*    (call-instruction---is-CALLCODE)        (call-instruction---current-frame---context-is-static ))
        (*    (call-instruction---is-DELEGATECALL)    (call-instruction---current-frame---context-is-static ))
        (*    (call-instruction---is-STATICCALL)      1                                                     )
        ))

(defun    (call-instruction---child-frame---account-address-hi)
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  (+    (*    (call-instruction---is-CALL)            (call-instruction---callee---address-hi                ))
        (*    (call-instruction---is-CALLCODE)        (call-instruction---current-frame---account-address-hi ))
        (*    (call-instruction---is-DELEGATECALL)    (call-instruction---current-frame---account-address-hi ))
        (*    (call-instruction---is-STATICCALL)      (call-instruction---callee---address-hi                ))
        ))

(defun    (call-instruction---child-frame---account-address-lo)
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  (+    (*    (call-instruction---is-CALL)            (call-instruction---callee---address-lo                ))
        (*    (call-instruction---is-CALLCODE)        (call-instruction---current-frame---account-address-lo ))
        (*    (call-instruction---is-DELEGATECALL)    (call-instruction---current-frame---account-address-lo ))
        (*    (call-instruction---is-STATICCALL)      (call-instruction---callee---address-lo                ))
        ))

(defun    (call-instruction---child-frame---account-deployment-number)
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  (+    (*    (call-instruction---is-CALL)            (shift    account/DEPLOYMENT_NUMBER    CALL_1st_callee_account_row___row_offset ))
        (*    (call-instruction---is-CALLCODE)        (call-instruction---current-frame---account-deployment-number                   ))
        (*    (call-instruction---is-DELEGATECALL)    (call-instruction---current-frame---account-deployment-number                   ))
        (*    (call-instruction---is-STATICCALL)      (shift    account/DEPLOYMENT_NUMBER    CALL_1st_callee_account_row___row_offset ))
        ))

(defun    (call-instruction---child-frame---caller-address-hi)
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  (+    (*    (call-instruction---is-CALL)            (call-instruction---current-frame---account-address-hi ))
        (*    (call-instruction---is-CALLCODE)        (call-instruction---current-frame---account-address-hi ))
        (*    (call-instruction---is-DELEGATECALL)    (call-instruction---current-frame---caller-address-hi  ))
        (*    (call-instruction---is-STATICCALL)      (call-instruction---current-frame---account-address-hi ))
        ))

(defun    (call-instruction---child-frame---caller-address-lo)
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  (+    (*    (call-instruction---is-CALL)            (call-instruction---current-frame---account-address-lo ))
        (*    (call-instruction---is-CALLCODE)        (call-instruction---current-frame---account-address-lo ))
        (*    (call-instruction---is-DELEGATECALL)    (call-instruction---current-frame---caller-address-lo  ))
        (*    (call-instruction---is-STATICCALL)      (call-instruction---current-frame---account-address-lo ))
        ))

(defun    (call-instruction---child-frame---call-value)
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  (+    (*    (call-instruction---is-CALL)            (call-instruction---STACK-value-lo             ))
        (*    (call-instruction---is-CALLCODE)        (call-instruction---STACK-value-lo             ))
        (*    (call-instruction---is-DELEGATECALL)    (call-instruction---current-frame---call-value ))
        (*    (call-instruction---is-STATICCALL)      0                                              )
        ))
