(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The USER-transaction case                          ;;
;;   X.Y.Z Transaction processing                           ;;
;;   X.Y.Z.T Final context row                              ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (tx-skip---coinbase-fee)    (shift    (*    transaction/PRIORITY_FEE_PER_GAS    (-    transaction/GAS_LIMIT    transaction/REFUND_EFFECTIVE))
                                                tx-skip---USER---row-offset---TXN))

(defconstraint   tx-skip---setting-coinbase-account-row
                 (:guard (tx-skip---precondition---USER))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (eq!    (shift account/ADDRESS_HI             tx-skip---USER---row-offset---ACC---coinbase)   (shift transaction/COINBASE_ADDRESS_HI   tx-skip---USER---row-offset---TXN))
                   (eq!    (shift account/ADDRESS_LO             tx-skip---USER---row-offset---ACC---coinbase)   (shift transaction/COINBASE_ADDRESS_LO   tx-skip---USER---row-offset---TXN))
                   (account-increment-balance-by                 tx-skip---USER---row-offset---ACC---coinbase    (tx-skip---coinbase-fee))
                   (account-same-nonce                           tx-skip---USER---row-offset---ACC---coinbase)
                   (account-same-code                            tx-skip---USER---row-offset---ACC---coinbase)
                   (account-same-deployment-number-and-status    tx-skip---USER---row-offset---ACC---coinbase)
                   (account-dont-check-for-delegation            tx-skip---USER---row-offset---ACC---coinbase)
                   (account-same-warmth                          tx-skip---USER---row-offset---ACC---coinbase)
                   (account-same-marked-for-deletion             tx-skip---USER---row-offset---ACC---coinbase)
                   (DOM-SUB-stamps---standard                    tx-skip---USER---row-offset---ACC---coinbase
                                                                 tx-skip---USER---row-offset---ACC---coinbase)))
