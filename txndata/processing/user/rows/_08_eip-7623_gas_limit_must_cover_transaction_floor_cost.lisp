(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                             ;;
;;    X. USER transaction processing                           ;;
;;    X.Y Common computations                                  ;;
;;    X.Y.Z Gas limit must cover the transaction floor cost    ;;
;;                                                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    USER-transaction---common-computations---gas-limit-and-transaction-floor-cost-comparison
                  (:guard   (first-row-of-USER-transaction))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (small-call-to-LEQ    ROFF___USER___CMPTN_ROW___GAS_LIMIT_MUST_COVER_THE_TRANSACTION_FLOOR_COST
                                        (USER-transaction---transaction-floor-cost)
                                        (USER-transaction---RLP---gas-limit))
                  )

(defconstraint    USER-transaction---common-computations---PRAGUE---gas-limit-must-cover-transaction-floor-cost
                   (:guard   (first-row-of-USER-transaction))
                   ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                   (result-must-be-true    ROFF___USER___CMPTN_ROW___GAS_LIMIT_MUST_COVER_THE_TRANSACTION_FLOOR_COST)
                   )

