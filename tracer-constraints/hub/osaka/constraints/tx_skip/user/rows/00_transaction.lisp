(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The USER-transaction case                          ;;
;;   X.Y.Z Transaction processing                           ;;
;;   X.Y.Z.T Transaction-row                                ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   tx-skip---TXN-row---justifying-requires-EVM-execution-is-zero-for-message-calls
                 (:guard (tx-skip---precondition---USER))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (vanishes!      (shift   transaction/REQUIRES_EVM_EXECUTION          tx-skip---USER---row-offset---TXN))
                   (if-not-zero    (tx-skip---USER---is-deployment)
                                   (vanishes!    (shift   transaction/INIT_CODE_SIZE    tx-skip---USER---row-offset---TXN)))
                   (if-not-zero    (tx-skip---USER---is-message-call)
                                   (eq!  (tx-skip---message-call-triggers-TX_SKIP)  1))
                   ))

(defun   (tx-skip---message-call-triggers-TX_SKIP)   (+   (tx-skip---RCPT---has-empty-code)
                                                          (tx-skip---RCPT---is-delegated-and-delegate-has-empty-code)
                                                          ;; (tx-skip---RCPT---is-delegated-and-delegate-is-delegated)
                                                          ))

(defun   (tx-skip---RCPT---is-delegated-and-delegate-has-empty-code)  (*  (tx-skip---RCPT---is-delegated)
                                                                          (tx-skip---DLGT---has-empty-code)
                                                                          ))

;; (defun   (tx-skip---RCPT---is-delegated-and-delegate-is-delegated)    (*  (tx-skip---RCPT---is-delegated)
;;                                                                           (tx-skip---DLGT---is-delegated)
;;                                                                           ))

(defun   (tx-skip---RCPT---has-empty-code)      (force-bin  (- 1 (tx-skip---RCPT---has-nonempty-code) )))
(defun   (tx-skip---DLGT---has-empty-code)      (force-bin  (- 1 (tx-skip---DLGT---has-nonempty-code) )))

(defun   (tx-skip---RCPT---has-nonempty-code)   (shift   account/HAS_CODE       tx-skip---USER---row-offset---ACC---recipient ) )
(defun   (tx-skip---DLGT---has-nonempty-code)   (shift   account/HAS_CODE       tx-skip---USER---row-offset---ACC---delegate  ) )

(defun   (tx-skip---RCPT---is-delegated)        (shift   account/IS_DELEGATED   tx-skip---USER---row-offset---ACC---recipient ))
(defun   (tx-skip---DLGT---is-delegated)        (shift   account/IS_DELEGATED   tx-skip---USER---row-offset---ACC---delegate  ))

(defun   (tx-skip---RCPT---isnt-delegated)      (force-bin  (-  1  (tx-skip---RCPT---is-delegated) )))
(defun   (tx-skip---DLGT---isnt-delegated)      (force-bin  (-  1  (tx-skip---DLGT---is-delegated) )))

(defconstraint   tx-skip---TXN-row---justifying-total-accrued-refunds---after-EIP-7702
                 (:guard (tx-skip---precondition---USER))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq!   (shift   transaction/REFUND_COUNTER_INFINITY   tx-skip---USER---row-offset---TXN)
                        REFUND_COUNTER
                        ))

(defconstraint   tx-skip---TXN-row---justifying-initial-balance
                 (:guard (tx-skip---precondition---USER))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq! (shift   account/BALANCE               tx-skip---USER---row-offset---ACC---sender)
                      (shift   transaction/INITIAL_BALANCE   tx-skip---USER---row-offset---TXN)))

(defconstraint   tx-skip---TXN-row---justifying-status-code
                 (:guard (tx-skip---precondition---USER))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq! (shift   transaction/STATUS_CODE tx-skip---USER---row-offset---TXN) 1))

;; NOTE: this constraint will be false starting with PRAGUE
;; specifically EIP-7702 (account delegation)
;; NOTE: this is now addressed using transaction/NUMBER_OF_SUCCESSFUL_SENDER_DELEGATIONS
(defconstraint   tx-skip---TXN-row---justifying-nonce
                 (:guard (tx-skip---precondition---USER))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq! (+  (shift   transaction/NONCE                                      tx-skip---USER---row-offset---TXN)
                          (shift   transaction/NUMBER_OF_SUCCESSFUL_SENDER_DELEGATIONS    tx-skip---USER---row-offset---TXN))
                      (shift   account/NONCE        tx-skip---USER---row-offset---ACC---sender)))

(defconstraint   tx-skip---TXN-row---justifying-left-over-gas
                 (:guard (tx-skip---precondition---USER))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq! (shift   transaction/GAS_LEFTOVER               tx-skip---USER---row-offset---TXN)
                      (shift   transaction/GAS_INITIALLY_AVAILABLE    tx-skip---USER---row-offset---TXN)))
