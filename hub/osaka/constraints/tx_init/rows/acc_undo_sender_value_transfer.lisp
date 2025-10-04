(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                 ;;
;;   X     TX_INIT phase                           ;;
;;   X.Y   Common constraints                      ;;
;;   X.Y.Z Undoing sender account value transfer   ;;
;;                                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   tx-init---account-row---sender-value-transfer---undoing-row
                 (:guard (tx-init---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (tx-init---transaction-failure-prediction)
                                (begin
                                  (account-same-address-as                       tx-init---row-offset---ACC---sender-value-transfer---undoing   tx-init---row-offset---ACC---sender-value-transfer)
                                  (account-undo-balance-update                   tx-init---row-offset---ACC---sender-value-transfer---undoing   tx-init---row-offset---ACC---sender-value-transfer)
                                  (account-same-nonce                            tx-init---row-offset---ACC---sender-value-transfer---undoing)
                                  (account-same-code                             tx-init---row-offset---ACC---sender-value-transfer---undoing)
                                  (account-same-deployment-number-and-status     tx-init---row-offset---ACC---sender-value-transfer---undoing)
                                  (account-same-warmth                           tx-init---row-offset---ACC---sender-value-transfer---undoing)
                                  (account-same-marked-for-deletion              tx-init---row-offset---ACC---sender-value-transfer---undoing)
                                  (account-isnt-precompile                       tx-init---row-offset---ACC---sender-value-transfer---undoing)
                                  (DOM-SUB-stamps---revert-with-child            tx-init---row-offset---ACC---sender-value-transfer---undoing
                                                                                 4
                                                                                 (tx-init---transaction-end-stamp)))))
