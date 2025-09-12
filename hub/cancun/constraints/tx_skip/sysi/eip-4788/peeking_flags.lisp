(module hub)



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The SYSI-transaction case                          ;;
;;   X.Y.Z EIP-4788 transactions                            ;;
;;   X.Y.Z.T Peeking flag setting                           ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    tx-skip---SYSI-4788---setting-peeking-flags-that-hold-unconditionally
                  (:guard (tx-skip---precondition---SYSI-4788))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!    (+    (shift    PEEK_AT_TRANSACTION    ROFF---tx-skip---SYSI-4788---TXN                                       )
                                (shift    PEEK_AT_ACCOUNT        ROFF---tx-skip---SYSI-4788---ACC---loading-the-beacon-root-account     ))
                          NSR---tx-skip---SYSI-4788---unconditional))


(defconstraint    tx-skip---SYSI-4788---setting-the-exact-peeking-flags---trivial-case
                  (:guard (tx-skip---precondition---SYSI-4788))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero   (tx-skip---SYSI-4788---sys-txn-is-trivial)
                                 (eq!    (+    (shift    PEEK_AT_TRANSACTION    ROFF---tx-skip---SYSI-4788---TXN                                     )
                                               (shift    PEEK_AT_ACCOUNT        ROFF---tx-skip---SYSI-4788---ACC---loading-the-beacon-root-account   )
                                               (shift    PEEK_AT_CONTEXT        ROFF---tx-skip---SYSI-4788---CON---final-zero-context---trivial-case ))
                                         NSR---tx-skip---SYSI-4788---trivial-case)))


(defconstraint    tx-skip---SYSI-4788---setting-the-exact-peeking-flags---nontrivial-case
                  (:guard (tx-skip---precondition---SYSI-4788))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero   (tx-skip---SYSI-4788---sys-txn-is-nontrivial)
                                 (eq!    (+    (shift    PEEK_AT_TRANSACTION    ROFF---tx-skip---SYSI-4788---TXN                                        )
                                               (shift    PEEK_AT_ACCOUNT        ROFF---tx-skip---SYSI-4788---ACC---loading-the-beacon-root-account      )
                                               (shift    PEEK_AT_STORAGE        ROFF---tx-skip---SYSI-4788---STO---storing-the-time-stamp               )
                                               (shift    PEEK_AT_STORAGE        ROFF---tx-skip---SYSI-4788---STO---storing-the-beacon-root              )
                                               (shift    PEEK_AT_CONTEXT        ROFF---tx-skip---SYSI-4788---CON---final-zero-context---nontrivial-case ))
                                         NSR---tx-skip---SYSI-4788---nontrivial-case)))
