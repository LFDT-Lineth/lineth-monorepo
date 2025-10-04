(module hub)



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The USER-transaction case                          ;;
;;   X.Y.Z Shorthands                                       ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defconst
  tx-skip---USER---row-offset---TXN                        0
  tx-skip---USER---row-offset---ACC---sender               1
  tx-skip---USER---row-offset---ACC---recipient            2
  tx-skip---USER---row-offset---ACC---coinbase             3
  tx-skip---USER---row-offset---CON---final-zero-context   4
  tx-skip---USER---NSR                                     5
  )


(defun    (tx-skip---USER---is-deployment)    (force-bin (shift transaction/IS_DEPLOYMENT    tx-skip---USER---row-offset---TXN)))


(defun    (tx-skip---precondition---USER)    (*   (-    TOTL_TXN_NUMBER    (prev    TOTL_TXN_NUMBER))
                                                  USER
                                                  TX_SKIP
                                                  ))
