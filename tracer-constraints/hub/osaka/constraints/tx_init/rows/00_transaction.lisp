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
                   (eq!           (shift   transaction/REQUIRES_EVM_EXECUTION   tx-init---row-offset---TXN)   1)
                   (if-not-zero   (tx-init---is-message-call)
                                  (eq!   (tx-init---non-skip-message-call)   1))
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
                 (eq!   (+  (shift   transaction/NONCE                                    tx-init---row-offset---TXN)
                            (shift   transaction/NUMBER_OF_SUCCESSFUL_SENDER_DELEGATIONS  tx-init---row-offset---TXN))
                        (shift   account/NONCE       tx-init---row-offset---ACC---sender-pay-for-gas)))

;; (defconstraint   tx-init---transaction-row---transactions-supporting-delegation-lists-must-trigger-the-TX_AUTH-phase
;;                  (:guard (tx-init---standard-precondition))
;;                  (if-not-zero   (shift   transaction/TRANSACTION_TYPE_SUPPORTS_DELEGATION_LISTS   tx-init---row-offset---TXN)
;;                                 (begin
;;                                   (eq!   (shift TX_AUTH               tx-init---row-offset---row-preceding-the-init-phase)   1)
;;                                   (eq!   (shift PEEK_AT_TRANSACTION   tx-init---row-offset---row-preceding-the-init-phase)   1)
;;                                   )))
