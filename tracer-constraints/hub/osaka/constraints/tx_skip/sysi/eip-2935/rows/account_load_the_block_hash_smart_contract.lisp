(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The SYSI-transaction case                          ;;
;;   X.Y.Z EIP-2935 transactions                            ;;
;;   X.Y.Z.T Transaction processing                         ;;
;;   X.Y.Z.T.U Load the block hash smart contract           ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    tx-skip---SYSI-2935---loading-the-block-hash-history-system-smart-contract
                  (:guard (tx-skip---precondition---SYSI-2935))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (account-trim-address   ROFF---tx-skip---SYSI-2935---ACC---loading-the-block-hash-history-account   ;; row offset
                                            HISTORY_STORAGE_ADDRESS_HI                                                        ;; high part of raw, potentially untrimmed address
                                            HISTORY_STORAGE_ADDRESS_LO)                                                       ;; low  part of raw, potentially untrimmed address
                    (eq!     (shift account/ADDRESS_HI             ROFF---tx-skip---SYSI-2935---ACC---loading-the-block-hash-history-account)     HISTORY_STORAGE_ADDRESS_HI)
                    (eq!     (shift account/ADDRESS_LO             ROFF---tx-skip---SYSI-2935---ACC---loading-the-block-hash-history-account)     HISTORY_STORAGE_ADDRESS_LO)
                    (account-same-balance                          ROFF---tx-skip---SYSI-2935---ACC---loading-the-block-hash-history-account)
                    (account-same-nonce                            ROFF---tx-skip---SYSI-2935---ACC---loading-the-block-hash-history-account)
                    (account-same-code                             ROFF---tx-skip---SYSI-2935---ACC---loading-the-block-hash-history-account)
                    (account-same-deployment-number-and-status     ROFF---tx-skip---SYSI-2935---ACC---loading-the-block-hash-history-account)
                    (account-same-warmth                           ROFF---tx-skip---SYSI-2935---ACC---loading-the-block-hash-history-account)
                    (account-same-marked-for-deletion              ROFF---tx-skip---SYSI-2935---ACC---loading-the-block-hash-history-account)
                    (DOM-SUB-stamps---standard                     ROFF---tx-skip---SYSI-2935---ACC---loading-the-block-hash-history-account
                                                                   ROFF---tx-skip---SYSI-2935---ACC---loading-the-block-hash-history-account)))
