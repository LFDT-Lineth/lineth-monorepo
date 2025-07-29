(module hub)



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The SYSI-transaction case                          ;;
;;   X.Y.Z EIP-4788 transactions                            ;;
;;   X.Y.Z.T Shorthands                                     ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defconst
  tx-skip---SYSI-4788---row-offset---TXN                                     0
  tx-skip---SYSI-4788---row-offset---ACC---loading-the-beacon-root-account   1
  tx-skip---SYSI-4788---row-offset---STO---storing-the-time-stamp            2
  tx-skip---SYSI-4788---row-offset---STO---storing-the-beacon-root           3
  tx-skip---SYSI-4788---row-offset---CON---final-zero-context                4
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  tx-skip---SYSI-4788---NSR                                                  5
  )

(defun (tx-skip---SYSI-4788---timestamp)            (shift   [ transaction/SYST_TXN_DATA   1 ]   tx-skip---SYSI-4788---row-offset---TXN)) ;; ""
(defun (tx-skip---SYSI-4788---timestamp-mod-8191)   (shift   [ transaction/SYST_TXN_DATA   2 ]   tx-skip---SYSI-4788---row-offset---TXN)) ;; ""
(defun (tx-skip---SYSI-4788---beacon-root-hi)       (shift   [ transaction/SYST_TXN_DATA   3 ]   tx-skip---SYSI-4788---row-offset---TXN)) ;; ""
(defun (tx-skip---SYSI-4788---beacon-root-lo)       (shift   [ transaction/SYST_TXN_DATA   4 ]   tx-skip---SYSI-4788---row-offset---TXN)) ;; ""


(defun    (tx-skip---precondition---SYSI-4788)    (*   (-  TOTL_TXN_NUMBER  (prev  TOTL_TXN_NUMBER))
                                                       SYSI
                                                       TX_SKIP
                                                       (shift    transaction/EIP_4788    tx-skip---SYSI-4788---row-offset---TXN)
                                                       ))
