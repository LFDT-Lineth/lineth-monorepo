(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                                ;;
;;    X. USER transaction processing                              ;;
;;    X.Y Specialized computations                                ;;
;;    X.Y.Z EIP-1559 comparing max_fee agains max_priority_fee    ;;
;;                                                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    USER-transaction---specialized-computations---EIP-1559---comparing-max-fee-vs-max-priority-fee
                  (:guard   (first-row-of-USER-transaction-with-EIP-1559-gas-semantics))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (small-call-to-LEQ     ROFF___USER___CMPTN_ROW___COMPARING_MAX_FEE_AND_MAX_PRIORITY_FEE
                                           (USER-transaction---RLP---max-priority-fee)
                                           (USER-transaction---RLP---max-fee))
                    (result-must-be-true   ROFF___USER___CMPTN_ROW___COMPARING_MAX_FEE_AND_MAX_PRIORITY_FEE)
                    ))

