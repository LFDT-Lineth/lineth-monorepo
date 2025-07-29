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


(defconstraint    tx-skip---SYSI-2935---loading-the-beacon-root-system-smart-contract
                  (:guard (tx-skip---precondition---SYSI-2935))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (account-trim-address   tx-skip---SYSI-2935---row-offset---ACC---loading-the-beacon-root-account   ;; row offset
                                           HISTORY_STORAGE_ADDRESS_HI                                                    ;; high part of raw, potentially untrimmed address
                                           HISTORY_STORAGE_ADDRESS_LO)                                                   ;; low  part of raw, potentially untrimmed address
                   (eq!     (shift account/ADDRESS_HI             tx-skip---SYSI-2935---row-offset---ACC---loading-the-beacon-root-account)     HISTORY_STORAGE_ADDRESS_HI)
                   (eq!     (shift account/ADDRESS_LO             tx-skip---SYSI-2935---row-offset---ACC---loading-the-beacon-root-account)     HISTORY_STORAGE_ADDRESS_LO)
                   (account-same-balance                          tx-skip---SYSI-2935---row-offset---ACC---loading-the-beacon-root-account)
                   (account-same-nonce                            tx-skip---SYSI-2935---row-offset---ACC---loading-the-beacon-root-account)
                   (account-same-code                             tx-skip---SYSI-2935---row-offset---ACC---loading-the-beacon-root-account)
                   (account-same-deployment-number-and-status     tx-skip---SYSI-2935---row-offset---ACC---loading-the-beacon-root-account)
                   (account-same-warmth                           tx-skip---SYSI-2935---row-offset---ACC---loading-the-beacon-root-account)
                   (account-same-marked-for-deletion              tx-skip---SYSI-2935---row-offset---ACC---loading-the-beacon-root-account)
                   (DOM-SUB-stamps---standard                     tx-skip---SYSI-2935---row-offset---ACC---loading-the-beacon-root-account
                                                                  tx-skip---SYSI-2935---row-offset---ACC---loading-the-beacon-root-account)))
