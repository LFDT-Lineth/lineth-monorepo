(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                            ;;
;;    X. USER transaction processing                          ;;
;;    X.Y Common computations                                 ;;
;;    X.Y.Z Cumulative gas must not exceed block gas limit    ;;
;;                                                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    USER-transaction-processing---common-computations---cumulative-gas-must-not-exceed-block-gas-limit
                  (:guard    (first-row-of-USER-transaction))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (small-call-to-LEQ     ROFF___USER___CMPTN_ROW___CUMULATIVE_GAS_CONSUMPTION_MUST_NOT_EXCEED_BLOCK_GAS_LIMIT
                                           GAS_CUMULATIVE
                                           (USER-transaction---HUB---block-gas-limit))
                    (result-must-be-true   ROFF___USER___CMPTN_ROW___CUMULATIVE_GAS_CONSUMPTION_MUST_NOT_EXCEED_BLOCK_GAS_LIMIT)
                    ))

