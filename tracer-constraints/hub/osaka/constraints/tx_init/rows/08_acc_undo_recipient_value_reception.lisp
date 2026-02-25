(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                     ;;
;;   X     TX_INIT phase                               ;;
;;   X.Y   Common constraints                          ;;
;;   X.Y.Z Undoing recipient account value reception   ;;
;;                                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   tx-init---account-row---recipient-value-reception---undoing-row
                 (:guard (tx-init---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (tx-init---transaction-failure-prediction)
                                (begin
                                  (account-same-address-as                       tx-init---row-offset---ACC---recipient-value-reception---undoing   tx-init---row-offset---ACC---recipient-value-reception)
                                  (account-undo-balance-update                   tx-init---row-offset---ACC---recipient-value-reception---undoing   tx-init---row-offset---ACC---recipient-value-reception)
                                  (account-undo-nonce-update                     tx-init---row-offset---ACC---recipient-value-reception---undoing   tx-init---row-offset---ACC---recipient-value-reception)
                                  (account-undo-code-update                      tx-init---row-offset---ACC---recipient-value-reception---undoing   tx-init---row-offset---ACC---recipient-value-reception)
                                  (account-dont-check-for-delegation             tx-init---row-offset---ACC---recipient-value-reception---undoing)
                                  (account-same-warmth                           tx-init---row-offset---ACC---recipient-value-reception---undoing)
                                  (account-undo-deployment-status-update         tx-init---row-offset---ACC---recipient-value-reception---undoing   tx-init---row-offset---ACC---recipient-value-reception)
                                  (account-same-marked-for-deletion              tx-init---row-offset---ACC---recipient-value-reception---undoing)
                                  (DOM-SUB-stamps---revert-with-child            tx-init---row-offset---ACC---recipient-value-reception---undoing
                                                                                 tx-init---row-offset---ACC---recipient-value-reception---undoing
                                                                                 (tx-init---transaction-end-stamp)))))
