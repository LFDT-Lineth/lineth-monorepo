(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                               ;;
;;    X. USER transaction processing             ;;
;;    X.Y Specialized computations               ;;
;;    X.Y.Z Computing the effective gas price    ;;
;;                                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    USER-transaction-processing---specialized-computations---computing-the-effective-gas-price
                  (:guard   (first-row-of-USER-transaction-with-EIP-1559-gas-semantics))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (small-call-to-LEQ    ROFF___USER___CMPTN_ROW___COMPUTING_THE_EFFECTIVE_GAS_PRICE
                                          (+    (USER-transaction---RLP---max-priority-fee)    (USER-transaction---HUB---basefee))
                                          (USER-transaction---RLP---max-fee)
                                          )))


(defun   (get-full-tip)   (force-bin   (shift   computation/WCP_RES   ROFF___USER___CMPTN_ROW___COMPUTING_THE_EFFECTIVE_GAS_PRICE)))


(defconstraint    USER-transaction-processing---specialized-computations---setting-the-gas-price-for-transactions-with-EIP-1559-gas-semantics
                  (:guard   (first-row-of-USER-transaction-with-EIP-1559-gas-semantics))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                           (if-not-zero (get-full-tip)
                                        (eq!   (USER-transaction---HUB---gas-price)   (+   (USER-transaction---RLP---max-priority-fee)    (USER-transaction---HUB---basefee)))
                                        (eq!   (USER-transaction---HUB---gas-price)        (USER-transaction---RLP---max-fee))))
