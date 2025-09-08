(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                 ;;
;;    X. USER transaction processing               ;;
;;    X.Y Common computations                      ;;
;;    X.Y.Z EIP-3860 mandated init code pricing    ;;
;;                                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    USER-transaction---common-computations---EIP-3860---mandated-init-code-pricing
                  (:guard   (first-row-of-USER-transaction))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (call-to-EUC    ROFF___USER___CMPTN_ROW___EIP-3860___INIT_CODE_WORD_PRICING
                                  (+   (USER-transaction---HUB---init-code-size)   WORD_SIZE_MO)
                                  WORD_SIZE)
                  )

(defun    (USER-transaction---init-code-words)   (shift   computation/EUC_QUOTIENT    ROFF___USER___CMPTN_ROW___EIP-3860___INIT_CODE_WORD_PRICING))
(defun    (USER-transaction---init-code-cost)    (*   GAS_CONST_INIT_CODE_WORD       (USER-transaction---init-code-words)))

