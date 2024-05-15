(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;   3.3 ABS_TX_NUM and BTC_NUM   ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; @Olivier is this whole section useless ? It it contained in TXN_DATA ?
;; No, it isn't. The logic is indeed in the TXN_DATA module, here we
;; simply make sure there are no unexpected jumps or so in the numbers

(defconstraint ABS-BTC-initial-vanishing (:domain {0})
               (begin
                 (vanishes! ABSOLUTE_TRANSACTION_NUMBER)
                 (debug (vanishes! BATCH_NUMBER))))

(defconstraint BTC_NUM-transaction-constancy ()
               (transaction-constancy BATCH_NUMBER))
