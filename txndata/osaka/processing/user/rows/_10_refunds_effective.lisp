(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                          ;;
;;    X. USER transaction processing        ;;
;;    X.Y Common computations               ;;
;;    X.Y.Z Effective refund computation    ;;
;;                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    USER-transaction-processing---common-computations---computing-effective-refund
                  (:guard    (first-row-of-USER-transaction))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (small-call-to-LT    ROFF___USER___CMPTN_ROW___EFFECTIVE_GAS_REFUND_COMPUTATION
                                       (USER-transaction---HUB---refund-counter-final)
                                       (USER-transaction---refund-limit)))

(defun    (USER-transaction---accrued-refunds-are-LT-than-refund-limit)    (shift      computation/WCP_RES   ROFF___USER___CMPTN_ROW___EFFECTIVE_GAS_REFUND_COMPUTATION))
(defun    (USER-transaction---leftover-gas-plus-refunds)                   (if-zero   (force-bin   (USER-transaction---accrued-refunds-are-LT-than-refund-limit))
                                                                                      (+   (USER-transaction---HUB---gas-leftover)    (USER-transaction---refund-limit))
                                                                                      (+   (USER-transaction---HUB---gas-leftover)    (USER-transaction---HUB---refund-counter-final))
                                                                                      ))
(defun    (USER-transaction---consumed-gas-after-refunds)    (-   (USER-transaction---RLP---gas-limit)
                                                                  (USER-transaction---leftover-gas-plus-refunds)))
