(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;   X     TX_FINL phase        ;;
;;   X.Y   Common constraints   ;;
;;   X.Y.Z Transaction row      ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    tx-finl---transaction-row---justifying-TXN_DATA-predictions
                 (:guard (tx-finl---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (eq!   (shift   transaction/STATUS_CODE               tx-finl---row-offset---TXN)   (tx-finl---transaction-success))
                   (eq!   (shift   transaction/REFUND_COUNTER_INFINITY   tx-finl---row-offset---TXN)   (shift   REFUND_COUNTER_NEW    tx-finl---row-offset---row-preceding-the-finl-phase))
                   (eq!   (shift   transaction/GAS_LEFTOVER              tx-finl---row-offset---TXN)   (shift   GAS_NEXT              tx-finl---row-offset---row-preceding-the-finl-phase))))
