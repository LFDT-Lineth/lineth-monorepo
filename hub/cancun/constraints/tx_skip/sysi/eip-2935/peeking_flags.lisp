(module hub)



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The SYSI-transaction case                          ;;
;;   X.Y.Z EIP-2935 transactions                            ;;
;;   X.Y.Z.T Peeking flag setting                           ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    tx-skip---SYSI-2935---setting-the-peeking-flags-that-hold-unconditionally
                  (:guard (tx-skip---precondition---SYSI-2935))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!    (+    (shift    PEEK_AT_TRANSACTION    tx-skip---SYSI-2935---row-offset---TXN                                   )
                                (shift    PEEK_AT_ACCOUNT        tx-skip---SYSI-2935---row-offset---ACC---loading-the-beacon-root-account ))
                          tx-skip---SYSI-2935---NSR---unconditional))


(defconstraint    tx-skip---SYSI-2935---setting-the-exact-peeking-flags---trivial-case
                  (:guard (tx-skip---precondition---SYSI-2935))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero   (tx-skip---SYSI-2935---sys-txn-is-trivial)
                                 (eq!    (+    (shift    PEEK_AT_TRANSACTION    tx-skip---SYSI-2935---row-offset---TXN                                     )
                                               (shift    PEEK_AT_ACCOUNT        tx-skip---SYSI-2935---row-offset---ACC---loading-the-beacon-root-account   )
                                               (shift    PEEK_AT_CONTEXT        tx-skip---SYSI-2935---row-offset---CON---final-zero-context---trivial-case ))
                                         tx-skip---SYSI-2935---NSR---trivial-case)))


(defconstraint    tx-skip---SYSI-2935---setting-the-exact-peeking-flags---nontrivial-case
                  (:guard (tx-skip---precondition---SYSI-2935))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero   (tx-skip---SYSI-2935---sys-txn-is-nontrivial)
                                 (eq!    (+    (shift    PEEK_AT_TRANSACTION    tx-skip---SYSI-2935---row-offset---TXN                                        )
                                               (shift    PEEK_AT_ACCOUNT        tx-skip---SYSI-2935---row-offset---ACC---loading-the-beacon-root-account      )
                                               (shift    PEEK_AT_STORAGE        tx-skip---SYSI-2935---row-offset---STO---storing-the-time-stamp               )
                                               (shift    PEEK_AT_CONTEXT        tx-skip---SYSI-2935---row-offset---CON---final-zero-context---nontrivial-case ))
                                         tx-skip---SYSI-2935---NSR---nontrivial-case)))
