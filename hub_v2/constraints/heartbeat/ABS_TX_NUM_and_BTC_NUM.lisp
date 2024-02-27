(module hub_v2)

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

;; ;; TODO: 
;; ;; Caused by:
;; ;;     0: at line 22: (begin...
;; ;;     1: expected at least 1 argument, but received 0
;; ;; make: *** [Makefile:111: zkevm.bin] Error 1
;; (defconstraint ABS-BTC-increments ()
;;                (begin
;;                  (debug (any! (will-remain-constant ABSOLUTE_TRANSACTION_NUMBER) (will-inc ABSOLUTE_TRANSACTION_NUMBER 1)))
;;                  (debug (any! (will-remain-constant BATCH_NUMBER) (will-inc BATCH_NUMBER 1)))))
