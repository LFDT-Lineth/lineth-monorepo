(module hub)



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The SYSI-transaction case                          ;;
;;   X.Y.Z EIP-2935 transactions                            ;;
;;   X.Y.Z.T Shorthands                                     ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defconst
  tx-skip---SYSI-2935---row-offset---TXN                                        0
  tx-skip---SYSI-2935---row-offset---ACC---loading-the-beacon-root-account      1
  tx-skip---SYSI-2935---row-offset---STO---storing-the-time-stamp               2
  tx-skip---SYSI-2935---row-offset---CON---final-zero-context---nontrivial-case 3
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  tx-skip---SYSI-2935---row-offset---CON---final-zero-context---trivial-case    2
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  tx-skip---SYSI-2935---NSR---unconditional                                     2
  tx-skip---SYSI-2935---NSR---trivial-case                                      3
  tx-skip---SYSI-2935---NSR---nontrivial-case                                   4
  )


(defun (tx-skip---SYSI-2935---sys-smc-has-code)             (shift   account/HAS_CODE              tx-skip---SYSI-2935---row-offset---ACC---loading-the-beacon-root-account)) ;; ""
;; we don't need "prev-block-number" itself
(defun (tx-skip---SYSI-2935---prev-block-number-mod-8191)   (shift   transaction/SYST_TXN_DATA_2   tx-skip---SYSI-2935---row-offset---TXN)) ;; ""
(defun (tx-skip---SYSI-2935---prev-block-hash-hi)           (shift   transaction/SYST_TXN_DATA_3   tx-skip---SYSI-2935---row-offset---TXN)) ;; ""
(defun (tx-skip---SYSI-2935---prev-block-hash-lo)           (shift   transaction/SYST_TXN_DATA_4   tx-skip---SYSI-2935---row-offset---TXN)) ;; ""
(defun (tx-skip---SYSI-2935---current-block-is-genesis)     (shift   transaction/SYST_TXN_DATA_5   tx-skip---SYSI-2935---row-offset---TXN)) ;; ""
(defun (tx-skip---SYSI-2935---current-block-isnt-genesis)   (force-bin (-  1  (tx-skip---SYSI-2935---current-block-is-genesis))))


(defun (tx-skip---SYSI-2935---sys-txn-is-nontrivial)      (force-bin (*  (tx-skip---SYSI-2935---sys-smc-has-code)
                                                                         (tx-skip---SYSI-2935---current-block-isnt-genesis))))
(defun (tx-skip---SYSI-2935---sys-txn-is-trivial)         (force-bin (-  1   (tx-skip---SYSI-2935---sys-txn-is-nontrivial))))


(defun    (tx-skip---precondition---SYSI-2935)    (force-bin (*   (-    TOTL_TXN_NUMBER    (prev    TOTL_TXN_NUMBER))
                                                                  SYSI
                                                                  TX_SKIP
                                                                  (shift    transaction/EIP_2935    tx-skip---SYSI-2935---row-offset---TXN)
                                                       )))
