(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                             ;;
;;    X. USER transaction processing                           ;;
;;    X.Y Common computations                                  ;;
;;    X.Y.Z EIP 7825 - Transaction Gas Limit Cap (>= Osaka)    ;;
;;                                                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    USER-transaction---common-computations---transaction-gas-limit-cap-comp
                  (:guard   (first-row-of-USER-transaction))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (small-call-to-LEQ    ROFF___USER___CMPTN_ROW___GAS_LIMIT_CAP
                                        (USER-transaction---RLP---gas-limit)
                                        EIP_7825_TRANSACTION_GAS_LIMIT_CAP)
                  )

(defconstraint    USER-transaction---common-computations---transaction-gas-limit-cap
                   (:guard   (first-row-of-USER-transaction))
                   ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                   (result-must-be-true    ROFF___USER___CMPTN_ROW___GAS_LIMIT_CAP)
                   )