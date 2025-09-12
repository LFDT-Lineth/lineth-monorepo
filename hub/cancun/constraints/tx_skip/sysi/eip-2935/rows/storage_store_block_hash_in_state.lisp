(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The SYSI-transaction case                          ;;
;;   X.Y.Z EIP-2935 transactions                            ;;
;;   X.Y.Z.T Transaction processing                         ;;
;;   X.Y.Z.T.U Store block hash in state                    ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    tx-skip---SYSI-2935---storing-the-previous-block-hash-in-state
                  (:guard (tx-skip---precondition---SYSI-2935))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero   (tx-skip---SYSI-2935---sys-txn-is-nontrivial)
                                 (begin
                                   (eq!   (shift   storage/ADDRESS_HI              ROFF---tx-skip---SYSI-2935---STO---storing-the-previous-block-hash)   HISTORY_STORAGE_ADDRESS_HI)
                                   (eq!   (shift   storage/ADDRESS_LO              ROFF---tx-skip---SYSI-2935---STO---storing-the-previous-block-hash)   HISTORY_STORAGE_ADDRESS_LO)
                                   (eq!   (shift   storage/DEPLOYMENT_NUMBER       ROFF---tx-skip---SYSI-2935---STO---storing-the-previous-block-hash)   (shift   account/DEPLOYMENT_NUMBER   ROFF---tx-skip---SYSI-2935---ACC---loading-the-block-hash-history-account))
                                   (eq!   (shift   storage/STORAGE_KEY_HI          ROFF---tx-skip---SYSI-2935---STO---storing-the-previous-block-hash)   0)
                                   (eq!   (shift   storage/STORAGE_KEY_LO          ROFF---tx-skip---SYSI-2935---STO---storing-the-previous-block-hash)   (tx-skip---SYSI-2935---prev-block-number-mod-8191))
                                   (eq!   (shift   storage/VALUE_NEXT_HI           ROFF---tx-skip---SYSI-2935---STO---storing-the-previous-block-hash)   (tx-skip---SYSI-2935---prev-block-hash-hi))
                                   (eq!   (shift   storage/VALUE_NEXT_LO           ROFF---tx-skip---SYSI-2935---STO---storing-the-previous-block-hash)   (tx-skip---SYSI-2935---prev-block-hash-lo))
                                   (storage-same-warmth                            ROFF---tx-skip---SYSI-2935---STO---storing-the-previous-block-hash)
                                   (DOM-SUB-stamps---standard                      ROFF---tx-skip---SYSI-2935---STO---storing-the-previous-block-hash       ;; kappa
                                                                                   ROFF---tx-skip---SYSI-2935---STO---storing-the-previous-block-hash))     ;; c
                                 ))

