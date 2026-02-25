(module hub)



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The USER-transaction case                          ;;
;;   X.Y.Z Shorthands                                       ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defconst
  tx-skip---USER---row-offset---row-preceding-the-TX_INIT-phase  -1
  tx-skip---USER---row-offset---TXN                               0
  tx-skip---USER---row-offset---ACC---sender                      1
  tx-skip---USER---row-offset---ACC---recipient                   2
  tx-skip---USER---row-offset---ACC---delegate                    3
  tx-skip---USER---row-offset---ACC---coinbase                    4
  tx-skip---USER---row-offset---CON---final-zero-context          5
  tx-skip---USER---NSR                                            (+ tx-skip---USER---row-offset---CON---final-zero-context 1)
  )


(defun    (tx-skip---USER---is-deployment)      (force-bin (shift transaction/IS_DEPLOYMENT    tx-skip---USER---row-offset---TXN)))
(defun    (tx-skip---USER---is-message-call)    (force-bin (-  1  (tx-skip---USER---is-deployment))))


(defun    (tx-skip---precondition---USER)    (*   (-    TOTL_TXN_NUMBER    (prev    TOTL_TXN_NUMBER))
                                                  USER
                                                  TX_SKIP
                                                  ))
