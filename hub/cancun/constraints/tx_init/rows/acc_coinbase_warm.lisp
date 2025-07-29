(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                            ;;
;;   X     TX_INIT phase                      ;;
;;   X.Y   Common constraints                 ;;
;;   X.Y.Z Coinbase warm                      ;;
;;                                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   tx-init---account-row---coinbase-warm
                 (:guard (tx-init---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (eq!     (shift account/ADDRESS_HI             tx-init---row-offset---ACC---coinbase-warming)     (tx-init---coinbase-address-hi))
                   (eq!     (shift account/ADDRESS_LO             tx-init---row-offset---ACC---coinbase-warming)     (tx-init---coinbase-address-lo))
                   (account-trim-address                          tx-init---row-offset---ACC---coinbase-warming      (tx-init---coinbase-address-hi) (tx-init---coinbase-address-lo))
                   (account-same-balance                          tx-init---row-offset---ACC---coinbase-warming)
                   (account-same-nonce                            tx-init---row-offset---ACC---coinbase-warming)
                   (account-same-code                             tx-init---row-offset---ACC---coinbase-warming)
                   (account-same-deployment-number-and-status     tx-init---row-offset---ACC---coinbase-warming)
                   (account-turn-on-warmth                        tx-init---row-offset---ACC---coinbase-warming)
                   (account-same-marked-for-deletion              tx-init---row-offset---ACC---coinbase-warming)
                   (DOM-SUB-stamps---standard                     tx-init---row-offset---ACC---coinbase-warming
                                                                  0)))
