(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;   3.3 ABS_TX_NUM and BTC_NUM   ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; @Olivier is this whole section useless ? It it contained in TXN_DATA ?
;; No, it isn't. The logic is indeed in the TXN_DATA module, here we
;; simply make sure there are no unexpected jumps or so in the numbers

(defconstraint    ABS-and-BLK-constraints---initial-vanishing (:domain {0})
                  ;; "initialization" constraints
                  (begin
                    (vanishes! ABSOLUTE_TRANSACTION_NUMBER)
                    (debug     (vanishes! RELATIVE_BLOCK_NUMBER))))

;; "increment" constraints
(defconstraint    ABS-and-BLK-constraints---increments       ()
                  (begin
                    (debug    (any!      (will-remain-constant!    ABSOLUTE_TRANSACTION_NUMBER)
                                         (will-inc!                ABSOLUTE_TRANSACTION_NUMBER    1)))
                    (debug    (any!      (will-remain-constant!    RELATIVE_BLOCK_NUMBER)
                                         (will-inc!                RELATIVE_BLOCK_NUMBER    1)))))
))

(defconstraint    ABS-and-BLK-constraints---transaction-constancy ()
                  (transaction-constancy RELATIVE_BLOCK_NUMBER))
