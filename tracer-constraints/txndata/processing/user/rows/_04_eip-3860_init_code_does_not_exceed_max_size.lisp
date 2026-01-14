(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                    ;;
;;    X. USER transaction processing                  ;;
;;    X.Y Common computations                         ;;
;;    X.Y.Z EIP-3860 mandated init code size check    ;;
;;                                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    USER-transaction---common-computations---EIP-3860---init-code-size-comparison-to-max-value
                  (:guard   (first-row-of-USER-transaction))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (small-call-to-LEQ    ROFF___USER___CMPTN_ROW___EIP-3860___MAX_INIT_CODE_SIZE_BOUND_CHECK
                                          (USER-transaction---HUB---init-code-size)
                                          MAX_INIT_CODE_SIZE)
                    (result-must-be-true    ROFF___USER___CMPTN_ROW___EIP-3860___MAX_INIT_CODE_SIZE_BOUND_CHECK)
                    ))
