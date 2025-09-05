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
  tx-skip---SYSI-4788---row-offset---TXN                                        0
  tx-skip---SYSI-4788---row-offset---ACC---loading-the-beacon-root-account      1
  tx-skip---SYSI-4788---row-offset---STO---storing-the-time-stamp               2
  tx-skip---SYSI-4788---row-offset---STO---storing-the-beacon-root              3
  tx-skip---SYSI-4788---row-offset---CON---final-zero-context---nontrivial-case 4
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  tx-skip---SYSI-4788---row-offset---CON---final-zero-context---trivial-case    2
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  tx-skip---SYSI-4788---NSR---unconditional                                     2
  tx-skip---SYSI-4788---NSR---trivial-case                                      3
  tx-skip---SYSI-4788---NSR---nontrivial-case                                   5
  )

(defun (tx-skip---SYSI-4788---sys-smc-has-code)             (shift   account/HAS_CODE              tx-skip---SYSI-4788---row-offset---ACC---loading-the-beacon-root-account)) ;; ""
(defun (tx-skip---SYSI-4788---timestamp)                    (shift   transaction/SYST_TXN_DATA_1   tx-skip---SYSI-4788---row-offset---TXN)) ;; ""
(defun (tx-skip---SYSI-4788---timestamp-mod-8191)           (shift   transaction/SYST_TXN_DATA_2   tx-skip---SYSI-4788---row-offset---TXN)) ;; ""
(defun (tx-skip---SYSI-4788---beacon-root-hi)               (shift   transaction/SYST_TXN_DATA_3   tx-skip---SYSI-4788---row-offset---TXN)) ;; ""
(defun (tx-skip---SYSI-4788---beacon-root-lo)               (shift   transaction/SYST_TXN_DATA_4   tx-skip---SYSI-4788---row-offset---TXN)) ;; ""
(defun (tx-skip---SYSI-4788---current-block-is-genesis)     (shift   transaction/SYST_TXN_DATA_5   tx-skip---SYSI-4788---row-offset---TXN)) ;; ""
(defun (tx-skip---SYSI-4788---current-block-isnt-genesis)   (force-bin (-  1  (tx-skip---SYSI-4788---current-block-is-genesis))))


(defun (tx-skip---SYSI-4788---sys-txn-is-nontrivial)      (force-bin (*  (tx-skip---SYSI-4788---sys-smc-has-code)
                                                                         (tx-skip---SYSI-4788---current-block-isnt-genesis))))
(defun (tx-skip---SYSI-4788---sys-txn-is-trivial)         (force-bin (-  1   (tx-skip---SYSI-4788---sys-txn-is-nontrivial))))


(defun    (tx-skip---precondition---SYSI-4788)    (force-bin (*   (-  TOTL_TXN_NUMBER  (prev  TOTL_TXN_NUMBER))
                                                                  SYSI
                                                                  TX_SKIP
                                                                  (shift    transaction/EIP_4788    tx-skip---SYSI-4788---row-offset---TXN)
                                                       )))
