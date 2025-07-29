(module hub)



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The SYSI-transaction case                          ;;
;;   X.Y.Z EIP-4788 transactions                            ;;
;;   X.Y.Z.T Peeking flag setting                           ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    tx-skip---SYSI-4788---setting-the-peeking-flags
                  (:guard (tx-skip---precondition---SYSI-4788))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!    (+    (shift    PEEK_AT_TRANSACTION    tx-skip---SYSI-4788---row-offset---TXN                                    )
                                (shift    PEEK_AT_ACCOUNT        tx-skip---SYSI-4788---row-offset---ACC---loading-the-beacon-root-account  )
                                (shift    PEEK_AT_STORAGE        tx-skip---SYSI-4788---row-offset---STO---storing-the-time-stamp           )
                                (shift    PEEK_AT_STORAGE        tx-skip---SYSI-4788---row-offset---STO---storing-the-beacon-root          )
                                (shift    PEEK_AT_CONTEXT        tx-skip---SYSI-4788---row-offset---CON---final-zero-context               ))
                          tx-skip---SYSI-4788---NSR))
