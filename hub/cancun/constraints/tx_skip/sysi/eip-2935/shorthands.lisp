(module hub)



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The SYSI-transaction case                          ;;
;;   X.Y.Z EIP-2935 transactions                            ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defconst
  tx-skip---SYSI-2935---row-offset---TXN                                     0
  tx-skip---SYSI-2935---row-offset---ACC---loading-the-beacon-root-account   1
  tx-skip---SYSI-2935---row-offset---STO---storing-the-time-stamp            2
  tx-skip---SYSI-2935---row-offset---CON---final-zero-context                3
  tx-skip---SYSI-2935---NSR                                                  4
  )


(defun (tx-skip---SYSI-2935---prev-block-number-mod-8191)   (shift   [ transaction/SYST_TXN_DATA   1 ]   tx-skip---SYSI-2935---row-offset---TXN)) ;; ""
(defun (tx-skip---SYSI-2935---prev-block-hash-hi)           (shift   [ transaction/SYST_TXN_DATA   2 ]   tx-skip---SYSI-2935---row-offset---TXN)) ;; ""
(defun (tx-skip---SYSI-2935---prev-block-hash-lo)           (shift   [ transaction/SYST_TXN_DATA   3 ]   tx-skip---SYSI-2935---row-offset---TXN)) ;; ""


(defun    (tx-skip---precondition---SYSI-2935)    (*   (-    TOTL_TXN_NUMBER    (prev    TOTL_TXN_NUMBER))
                                                       SYSI
                                                       TX_SKIP
                                                       (shift    transaction/EIP_2935    tx-skip---SYSI-2935---row-offset---TXN)
                                                       ))
