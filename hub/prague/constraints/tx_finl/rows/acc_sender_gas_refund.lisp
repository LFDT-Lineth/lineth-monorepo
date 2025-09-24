(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;   X     TX_FINL phase        ;;
;;   X.Y   Common constraints   ;;
;;   X.Y.Z Sender gas refund    ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   tx-finl---account-row---sender-gas-refund
                 (:guard (tx-finl---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (eq!     (shift account/ADDRESS_HI             tx-finl---row-offset---ACC---sender-gas-refund)     (tx-finl---sender-address-hi))
                   (eq!     (shift account/ADDRESS_LO             tx-finl---row-offset---ACC---sender-gas-refund)     (tx-finl---sender-address-lo))
                   (account-increment-balance-by                  tx-finl---row-offset---ACC---sender-gas-refund      (tx-finl---sender-gas-refund))
                   (account-same-nonce                            tx-finl---row-offset---ACC---sender-gas-refund)
                   (account-same-code                             tx-finl---row-offset---ACC---sender-gas-refund)
                   (account-same-deployment-number-and-status     tx-finl---row-offset---ACC---sender-gas-refund)
                   (account-same-warmth                           tx-finl---row-offset---ACC---sender-gas-refund)
                   (account-same-marked-for-deletion              tx-finl---row-offset---ACC---sender-gas-refund)
                   (DOM-SUB-stamps---standard                     tx-finl---row-offset---ACC---sender-gas-refund
                                                                  0)))
