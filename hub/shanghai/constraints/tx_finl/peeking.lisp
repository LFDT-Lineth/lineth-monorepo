(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                   ;;
;;   X   TX_FINL phase               ;;
;;   X.Y Introduction                ;;
;;   X.Y Setting the peeking flags   ;;
;;                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    tx-finl---setting-peeking-flags
                  (:guard    (tx-finl---standard-precondition))
                  (eq!   3
                         (+ (shift   PEEK_AT_ACCOUNT       tx-finl---row-offset---ACC---sender-gas-refund)
                            (shift   PEEK_AT_ACCOUNT       tx-finl---row-offset---ACC---coinbase-reward)
                            (shift   PEEK_AT_TRANSACTION   tx-finl---row-offset---TXN))))
