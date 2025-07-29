(module hub)



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The USER-transaction case                          ;;
;;   X.Y.Z Peeking flag setting                             ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    tx-skip---USER---setting-the-peeking-flags
                  (:guard (tx-skip---precondition---USER))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!    (+   (shift    PEEK_AT_TRANSACTION   tx-skip---USER---row-offset---TXN                      )
                               (shift    PEEK_AT_ACCOUNT       tx-skip---USER---row-offset---ACC---sender             )
                               (shift    PEEK_AT_ACCOUNT       tx-skip---USER---row-offset---ACC---recipient          )
                               (shift    PEEK_AT_ACCOUNT       tx-skip---USER---row-offset---ACC---coinbase           )
                               (shift    PEEK_AT_CONTEXT       tx-skip---USER---row-offset---CON---final-zero-context ))
                          tx-skip---USER---NSR))                                               

