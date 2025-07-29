(module hub)



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The SYSI-transaction case                          ;;
;;   X.Y.Z EIP-2935 transactions                            ;;
;;   X.Y.Z.T Peeking flag setting                           ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    tx-skip---SYSI-2935---setting-the-peeking-flags
                  (:guard (tx-skip---precondition---SYSI-2935))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!    (+    (shift    PEEK_AT_TRANSACTION    tx-skip---SYSI-2935---row-offset---TXN                                   )
                                (shift    PEEK_AT_ACCOUNT        tx-skip---SYSI-2935---row-offset---ACC---loading-the-beacon-root-account )
                                (shift    PEEK_AT_STORAGE        tx-skip---SYSI-2935---row-offset---STO---storing-the-time-stamp          )
                                (shift    PEEK_AT_CONTEXT        tx-skip---SYSI-2935---row-offset---CON---final-zero-context              ))
                          tx-skip---SYSI-2935---NSR))                                               
