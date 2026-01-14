(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                      ;;
;;    X. USER transaction processing    ;;
;;    X.Y Common computations           ;;
;;    X.Y.Z Upper limit for refunds     ;;
;;                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    USER-transaction-processing---common-computations---upper-limit-for-refunds
                  (:guard    (first-row-of-USER-transaction))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (call-to-EUC   ROFF___USER___CMPTN_ROW___UPPER_LIMIT_FOR_GAS_REFUNDS
                                 (USER-transaction---execution-gas-cost)
                                 MAX_REFUND_QUOTIENT))


(defun    (USER-transaction---execution-gas-cost)    (-   (USER-transaction---RLP---gas-limit)
                                                          (USER-transaction---HUB---gas-leftover)))

(defun    (USER-transaction---refund-limit)    (shift   computation/EUC_QUOTIENT   ROFF___USER___CMPTN_ROW___UPPER_LIMIT_FOR_GAS_REFUNDS))

