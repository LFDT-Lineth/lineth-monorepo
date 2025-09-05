(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The SYSI-transaction case                          ;;
;;   X.Y.Z EIP-4788 transactions                            ;;
;;   X.Y.Z.T Transaction processing                         ;;
;;   X.Y.Z.T.U Storing the timestamp in the state           ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    tx-skip---SYSI-4788---storing-the-timestamp-in-the-state
                  (:guard (tx-skip---precondition---SYSI-4788))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero   (tx-skip---SYSI-4788---sys-txn-is-nontrivial)
                                 (begin
                                   (eq!   (shift   storage/ADDRESS_HI          tx-skip---SYSI-4788---row-offset---STO---storing-the-time-stamp)   BEACON_ROOTS_ADDRESS_HI)
                                   (eq!   (shift   storage/ADDRESS_LO          tx-skip---SYSI-4788---row-offset---STO---storing-the-time-stamp)   BEACON_ROOTS_ADDRESS_LO)
                                   (eq!   (shift   storage/DEPLOYMENT_NUMBER   tx-skip---SYSI-4788---row-offset---STO---storing-the-time-stamp)   (shift   account/DEPLOYMENT_NUMBER   tx-skip---SYSI-4788---row-offset---ACC---loading-the-beacon-root-account))
                                   (eq!   (shift   storage/STORAGE_KEY_HI      tx-skip---SYSI-4788---row-offset---STO---storing-the-time-stamp)   0)
                                   (eq!   (shift   storage/STORAGE_KEY_LO      tx-skip---SYSI-4788---row-offset---STO---storing-the-time-stamp)   (tx-skip---SYSI-4788---timestamp-mod-8191))
                                   (eq!   (shift   storage/VALUE_NEXT_HI       tx-skip---SYSI-4788---row-offset---STO---storing-the-time-stamp)   0)
                                   (eq!   (shift   storage/VALUE_NEXT_LO       tx-skip---SYSI-4788---row-offset---STO---storing-the-time-stamp)   (tx-skip---SYSI-4788---timestamp))
                                   (storage-same-warmth                        tx-skip---SYSI-4788---row-offset---STO---storing-the-time-stamp)
                                   (DOM-SUB-stamps---standard                  tx-skip---SYSI-4788---row-offset---STO---storing-the-time-stamp       ;; kappa
                                                                               tx-skip---SYSI-4788---row-offset---STO---storing-the-time-stamp))     ;; c
                                 ))

