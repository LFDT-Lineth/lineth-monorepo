(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The USER-transaction case                          ;;
;;   X.Y.Z Transaction processing                           ;;
;;   X.Y.Z.T Sender account row                             ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defconstraint   tx-skip---setting-sender-account-row
                 (:guard (tx-skip---precondition---USER))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (eq!    (shift account/ADDRESS_HI                          tx-skip---USER---row-offset---ACC---sender)
                           (shift transaction/FROM_ADDRESS_HI                 tx-skip---USER---row-offset---TXN))
                   (eq!    (shift account/ADDRESS_LO                          tx-skip---USER---row-offset---ACC---sender)
                           (shift transaction/FROM_ADDRESS_LO                 tx-skip---USER---row-offset---TXN))
                   (account-decrement-balance-by                              tx-skip---USER---row-offset---ACC---sender    (tx-skip---USER---wei-cost-for-sender))
                   (account-increment-nonce                                   tx-skip---USER---row-offset---ACC---sender)
                   (account-same-code                                         tx-skip---USER---row-offset---ACC---sender)
                   (account-same-deployment-number-and-status                 tx-skip---USER---row-offset---ACC---sender)
                   (account-check-for-delegation-if-account-has-code          tx-skip---USER---row-offset---ACC---sender)
                   (account-same-warmth                                       tx-skip---USER---row-offset---ACC---sender)
                   (account-same-marked-for-deletion                          tx-skip---USER---row-offset---ACC---sender)
                   (account-isnt-precompile                                   tx-skip---USER---row-offset---ACC---sender)
                   (DOM-SUB-stamps---standard                                 tx-skip---USER---row-offset---ACC---sender
                                                                              tx-skip---USER---row-offset---ACC---sender)))



(defun   (tx-skip---USER---wei-cost-for-sender)   (shift    (+   transaction/VALUE
                                                                 (*      transaction/GAS_PRICE
                                                                         (-   transaction/GAS_LIMIT transaction/REFUND_EFFECTIVE)))
                                                            tx-skip---USER---row-offset---TXN))



(defconstraint     tx-skip---USER---reject-transactions-from-senders-with-deployed-code---EIP-3607
                   (:guard (tx-skip---precondition---USER))
                   ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                   (eq!  (tx-skip---SNDR---has-empty-code-or-is-delegated)  1))


(defun   (tx-skip---SNDR---has-empty-code-or-is-delegated)  (+  (tx-skip---SNDR---has-empty-code)
                                                                (tx-skip---SNDR---is-delegated)
                                                                ))

(defun   (tx-skip---SNDR---has-empty-code)     (force-bin  (-  1  (tx-skip---SNDR---has-nonempty-code))))

(defun   (tx-skip---SNDR---is-delegated)       (shift    account/IS_DELEGATED    tx-skip---USER---row-offset---ACC---sender) )
(defun   (tx-skip---SNDR---has-nonempty-code)  (shift    account/HAS_CODE        tx-skip---USER---row-offset---ACC---sender) )

