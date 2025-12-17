(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                         ;;
;;    X. USER transaction processing                       ;;
;;    X.Y Common computations                              ;;
;;    X.Y.Z Comparing the maximum gas price and basefee    ;;
;;                                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    USER-transaction-processing---common-computations---comparing-the-maximum-gas-price-to-the-basefee
                  (:guard    (first-row-of-USER-transaction))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (small-call-to-LEQ     ROFF___USER___CMPTN_ROW___THE_MAXIMUM_GAS_PRICE_MUST_MATCH_OR_EXCEED_THE_BASEFEE
                                           (USER-transaction---HUB---basefee)
                                           (USER-transaction---max-gas-price))
                    (result-must-be-true   ROFF___USER___CMPTN_ROW___THE_MAXIMUM_GAS_PRICE_MUST_MATCH_OR_EXCEED_THE_BASEFEE)
                    ))


