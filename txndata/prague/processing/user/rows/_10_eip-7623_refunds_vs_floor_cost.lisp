(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                                 ;;
;;    X. USER transaction processing                               ;;
;;    X.Y Common computations                                      ;;
;;    X.Y.Z Comparing effective refun to transaction floor cost    ;;
;;                                                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    USER-transaction-processing---common-computations---comparing-consumed-gas-after-refunds-with-transaction-floor-cost
                  (:guard    (first-row-of-USER-transaction))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (small-call-to-LT    ROFF___USER___CMPTN_ROW___EFFECTIVE_GAS_REFUND_VS_TRANSACTION_CALL_DATA_FLOOR_PRICE_COMPARISON
                                       (USER-transaction---consumed-gas-after-refunds)
                                       (USER-transaction---transaction-floor-cost)
                                       ))

(defun    (USER-transaction---pay-floor-price)    (shift    computation/WCP_RES    ROFF___USER___CMPTN_ROW___EFFECTIVE_GAS_REFUND_VS_TRANSACTION_CALL_DATA_FLOOR_PRICE_COMPARISON))

;; Prague specific
(defconstraint    USER-transaction-processing---common-computations---setting-REFUND_EFFECTIVE---PRAGUE-version
                  (:guard    (first-row-of-USER-transaction))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-zero    (force-bin    (USER-transaction---pay-floor-price))
                              ;; pay_floor_cost ≡ <false>
                              (eq!    (USER-transaction---HUB---refund-effective)
                                      (-   (USER-transaction---RLP---gas-limit)
                                           (USER-transaction---consumed-gas-after-refunds)))
                              ;; pay_floor_cost ≡ <true
                              (eq!    (USER-transaction---HUB---refund-effective)
                                      (-   (USER-transaction---RLP---gas-limit)
                                           (USER-transaction---transaction-floor-cost)))
                              ))
