(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                  ;;
;;   4.8 Setting HUB_STAMP_TX_END   ;;
;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint setting-HUB_STAMP_TX_END ()
               (begin
                 (transaction-constancy HUB_STAMP_TRANSACTION_END)
                 (if-not-zero TX_EXEC
                              (if-not-zero (next TX_FINL)
                                           (eq! HUB_STAMP_TRANSACTION_END
                                                (+ 1 HUB_STAMP))))))
