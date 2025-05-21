(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;   X     TX_INIT phase        ;;
;;   X.Y   Common constraints   ;;
;;   X.Y.Z Transaction row      ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   tx-init---transaction-row---partially-justifying-requires-evm-execution
                 (:guard (tx-init---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (eq!           1   (shift   transaction/REQUIRES_EVM_EXECUTION   tx-init---row-offset---TXN))
                   (if-not-zero   (tx-init---is-message-call)
                                  (eq!   1   (shift   account/HAS_CODE   tx-init---row-offset---ACC---recipient-value-reception)))
                   (if-not-zero   (tx-init---is-deployment)
                                  (is-not-zero!   (tx-init---init-code-size)))))

(defconstraint   tx-init---transaction-row---justifying-initial-balance
                 (:guard (tx-init---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq!   (shift   account/BALANCE               tx-init---row-offset---ACC---sender-pay-for-gas)
                        (shift   transaction/INITIAL_BALANCE   tx-init---row-offset---TXN)))

(defconstraint   tx-init---transaction-row---justifying-status-code
                 (:guard (tx-init---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq!   (shift   transaction/STATUS_CODE       tx-init---row-offset---TXN)
                        (tx-init---transaction-success-prediction)))

(defconstraint   tx-init---transaction-row---justifying-nonce
                 (:guard (tx-init---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq!   (shift   transaction/NONCE   tx-init---row-offset---TXN)
                        (shift   account/NONCE       tx-init---row-offset---ACC---sender-pay-for-gas)))
