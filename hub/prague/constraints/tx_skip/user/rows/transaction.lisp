(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The USER-transaction case                          ;;
;;   X.Y.Z Transaction processing                           ;;
;;   X.Y.Z.T Transaction row                                ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   tx-skip---TXN-row---partially-justifying-requires-evm-execution
                 (:guard (tx-skip---precondition---USER))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (vanishes!    (shift   transaction/REQUIRES_EVM_EXECUTION          tx-skip---USER---row-offset---TXN))
                   (if-zero      (tx-skip---USER---is-deployment)
                                 (vanishes!    (shift   account/HAS_CODE              tx-skip---USER---row-offset---ACC---recipient))
                                 (vanishes!    (shift   transaction/INIT_CODE_SIZE    tx-skip---USER---row-offset---TXN)))))

(defconstraint   tx-skip---TXN-row---justifying-total-accrued-refunds
                 (:guard (tx-skip---precondition---USER))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (vanishes! (shift   transaction/REFUND_COUNTER_INFINITY   tx-skip---USER---row-offset---TXN)))

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
;; modify for Prague-v2
(defconstraint   tx-skip---TXN-row---justifying-nonce
                 (:guard (tx-skip---precondition---USER))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq! (shift   transaction/NONCE    tx-skip---USER---row-offset---TXN)
                      (shift   account/NONCE        tx-skip---USER---row-offset---ACC---sender)))

(defconstraint   tx-skip---TXN-row---justifying-left-over-gas
                 (:guard (tx-skip---precondition---USER))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq! (shift   transaction/GAS_LEFTOVER               tx-skip---USER---row-offset---TXN)
                      (shift   transaction/GAS_INITIALLY_AVAILABLE    tx-skip---USER---row-offset---TXN)))

