(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                    ;;
;;   X     TX_INIT phase              ;;
;;   X.Y   Common constraints         ;;
;;   X.Y.Z Sender pays for gas_cost   ;;
;;                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   tx-init---account-row---sender-pays-for-gas
                 (:guard (tx-init---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (eq!     (shift account/ADDRESS_HI             tx-init---row-offset---ACC---sender-pay-for-gas)     (tx-init---sender-address-hi))
                   (eq!     (shift account/ADDRESS_LO             tx-init---row-offset---ACC---sender-pay-for-gas)     (tx-init---sender-address-lo))
                   (account-trim-address                          tx-init---row-offset---ACC---sender-pay-for-gas
                                                                  (tx-init---sender-address-hi)
                                                                  (tx-init---sender-address-lo))
                   (account-decrement-balance-by                  tx-init---row-offset---ACC---sender-pay-for-gas      (tx-init---gas-cost))
                   (account-increment-nonce                       tx-init---row-offset---ACC---sender-pay-for-gas)
                   (account-same-code                             tx-init---row-offset---ACC---sender-pay-for-gas)
                   (account-same-deployment-number-and-status     tx-init---row-offset---ACC---sender-pay-for-gas)
                   (account-turn-on-warmth                        tx-init---row-offset---ACC---sender-pay-for-gas)
                   (account-same-marked-for-deletion              tx-init---row-offset---ACC---sender-pay-for-gas)
                   (account-isnt-precompile                       tx-init---row-offset---ACC---sender-pay-for-gas)
                   (DOM-SUB-stamps---standard                     tx-init---row-offset---ACC---sender-pay-for-gas
                                                                  1)))

(defconstraint   tx-init---EIP-3607---reject-transactions-from-senders-with-deployed-code
                 (:guard (tx-init---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (vanishes!    (shift    account/HAS_CODE    tx-init---row-offset---ACC---sender-pay-for-gas)))
