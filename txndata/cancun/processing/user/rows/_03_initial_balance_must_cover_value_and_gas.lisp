(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                      ;;
;;    X. USER transaction processing                    ;;
;;    X.Y Common computations                           ;;
;;    X.Y.Z Initial balance must cover value and gas    ;;
;;                                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defconstraint    USER-transaction---common-computations---initial-balance-must-cover-value-and-gas
                  (:guard   (first-row-of-USER-transaction))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (small-call-to-LEQ     ROFF___USER___CMPTN_ROW___INITIAL_BALANCE_MUST_COVER_VALUE_AND_GAS
                                           (USER-transaction---max-cost-in-wei)
                                           (USER-transaction---HUB---initial-balance))
                    (result-must-be-true   ROFF___USER___CMPTN_ROW___INITIAL_BALANCE_MUST_COVER_VALUE_AND_GAS)
                    ))

(defun    (USER-transaction---max-cost-in-wei)
  (+   (USER-transaction---RLP---value)
       (*   (USER-transaction---RLP---gas-limit)
            (USER-transaction---max-gas-price))))

(defun    (USER-transaction---max-gas-price)
  (+    (*   (USER-transaction---tx-decoding---tx-type-with-fixed-gas-price)    (USER-transaction---RLP---gas-price))
        (*   (USER-transaction---tx-decoding---tx-type-sans-fixed-gas-price)    (USER-transaction---RLP---max-fee))))
