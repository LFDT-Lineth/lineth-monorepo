(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;   X     TX_INIT phase        ;;
;;   X.Y   Common constraints   ;;
;;   X.Y.Z Transaction row      ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   tx-init---account-row---sender-value-transfer
                 (:guard (tx-init---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (account-same-address-as                       tx-init---row-offset---ACC---sender-value-transfer
                                                                  tx-init---row-offset---ACC---sender-pay-for-gas)
                   (account-decrement-balance-by                  tx-init---row-offset---ACC---sender-value-transfer      (tx-init---value))
                   (account-same-nonce                            tx-init---row-offset---ACC---sender-value-transfer)
                   (account-same-code                             tx-init---row-offset---ACC---sender-value-transfer)
                   (account-same-deployment-number-and-status     tx-init---row-offset---ACC---sender-value-transfer)
                   (account-same-warmth                           tx-init---row-offset---ACC---sender-value-transfer)
                   (account-same-marked-for-deletion              tx-init---row-offset---ACC---sender-value-transfer)
                   (account-isnt-precompile                       tx-init---row-offset---ACC---sender-value-transfer)
                   (DOM-SUB-stamps---standard                     tx-init---row-offset---ACC---sender-value-transfer
                                                                  2)))
