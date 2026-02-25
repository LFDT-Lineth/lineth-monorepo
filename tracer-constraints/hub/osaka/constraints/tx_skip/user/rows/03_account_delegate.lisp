(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The USER-transaction case                          ;;
;;   X.Y.Z Transaction processing                           ;;
;;   X.Y.Z.T Delegate account-row                           ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun   (tx-skip---load-delegate-account)    (*  (tx-skip---USER---is-message-call)
                                                  (tx-skip---RCPT---is-delegated)
                                                  ))

(defun   (tx-skip---load-recipient-account-again)   (+  (*  (tx-skip---USER---is-message-call)
                                                            (tx-skip---RCPT---isnt-delegated)
                                                            )
                                                        (tx-skip---USER---is-deployment)
                                                        ))

;;--------------------------------;;
;;   The ``load delegate'' case   ;;
;;--------------------------------;;



(defconstraint   tx-skip---USER---delegate-account-row---the-load-DLGT-case---setting-address
                 (:guard (tx-skip---precondition---USER))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (tx-skip---load-delegate-account)
                                (begin
                                  (eq!   (shift   account/ADDRESS_HI              tx-skip---USER---row-offset---ACC---delegate  )
                                         (shift   account/DELEGATION_ADDRESS_HI   tx-skip---USER---row-offset---ACC---recipient ))
                                  (eq!   (shift   account/ADDRESS_LO              tx-skip---USER---row-offset---ACC---delegate  )
                                         (shift   account/DELEGATION_ADDRESS_LO   tx-skip---USER---row-offset---ACC---recipient ))
                                  )))

(defconstraint   tx-skip---USER---delegate-account-row---the-load-DLGT-case---setting-check-for-delegation
                 (:guard (tx-skip---precondition---USER))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (tx-skip---load-delegate-account)
                                (account-check-for-delegation-if-account-has-code   tx-skip---USER---row-offset---ACC---delegate)
                                ))


;;------------------------------------;;
;;   The ``re-load recipient'' case   ;;
;;------------------------------------;;



(defconstraint   tx-skip---USER---delegate-account-row---the-re-load-RCPT-case---setting-address
                 (:guard (tx-skip---precondition---USER))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (tx-skip---load-recipient-account-again)
                                (account-same-address-as   tx-skip---USER---row-offset---ACC---delegate   tx-skip---USER---row-offset---ACC---recipient)
                                ))

(defconstraint   tx-skip---USER---delegate-account-row---the-re-load-RCPT-case---setting-check-for-delegation
                 (:guard (tx-skip---precondition---USER))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (tx-skip---load-recipient-account-again)
                                (account-dont-check-for-delegation   tx-skip---USER---row-offset---ACC---delegate)
                                ))


(defconstraint   tx-skip---USER---setting-delegate-account-row
                 (:guard (tx-skip---precondition---USER))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   ;; (eq!    (shift account/ADDRESS_HI                        tx-skip---USER---row-offset---ACC---delegate)   (shift transaction/COINBASE_ADDRESS_HI    tx-skip---USER---row-offset---TXN))
                   ;; (eq!    (shift account/ADDRESS_LO                        tx-skip---USER---row-offset---ACC---delegate)   (shift transaction/COINBASE_ADDRESS_LO    tx-skip---USER---row-offset---TXN))
                   (account-same-balance                                    tx-skip---USER---row-offset---ACC---delegate)
                   (account-same-nonce                                      tx-skip---USER---row-offset---ACC---delegate)
                   (account-same-code                                       tx-skip---USER---row-offset---ACC---delegate)
                   (account-same-deployment-number-and-status               tx-skip---USER---row-offset---ACC---delegate)
                   (account-check-for-delegation-if-account-has-code        tx-skip---USER---row-offset---ACC---delegate)
                   (account-same-warmth                                     tx-skip---USER---row-offset---ACC---delegate)
                   (account-same-marked-for-deletion                        tx-skip---USER---row-offset---ACC---delegate)
                   ;; (account-isnt-precompile tx-skip---USER---row-offset---ACC---delegate) ;; the delegate address may be that of a precompile
                   (DOM-SUB-stamps---standard                               tx-skip---USER---row-offset---ACC---delegate
                                                                            tx-skip---USER---row-offset---ACC---delegate)
                   ))

(defun   (condition-for-checking-recipient-for-delegation)   (*   (tx-skip---load-delegate-account)
                                                                  (tx-skip---DLGT---has-nonempty-code)
                                                                  ))
