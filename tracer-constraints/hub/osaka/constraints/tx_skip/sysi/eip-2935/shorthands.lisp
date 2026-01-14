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
  ROFF---tx-skip---SYSI-2935---TXN                                          0
  ROFF---tx-skip---SYSI-2935---ACC---loading-the-block-hash-history-account 1
  ROFF---tx-skip---SYSI-2935---STO---storing-the-previous-block-hash        2
  ROFF---tx-skip---SYSI-2935---CON---final-zero-context---nontrivial-case   3
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ROFF---tx-skip---SYSI-2935---CON---final-zero-context---trivial-case      2
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  NSR---tx-skip---SYSI-2935---unconditional                                 2
  NSR---tx-skip---SYSI-2935---trivial-case                                  3
  NSR---tx-skip---SYSI-2935---nontrivial-case                               4
  )


(defun (tx-skip---SYSI-2935---sys-smc-has-code)             (shift   account/HAS_CODE              ROFF---tx-skip---SYSI-2935---ACC---loading-the-block-hash-history-account)) ;; ""
;; we don't need "prev-block-number" itself
(defun (tx-skip---SYSI-2935---prev-block-number-mod-8191)   (shift   transaction/SYST_TXN_DATA_2   ROFF---tx-skip---SYSI-2935---TXN)) ;; ""
(defun (tx-skip---SYSI-2935---prev-block-hash-hi)           (shift   transaction/SYST_TXN_DATA_3   ROFF---tx-skip---SYSI-2935---TXN)) ;; ""
(defun (tx-skip---SYSI-2935---prev-block-hash-lo)           (shift   transaction/SYST_TXN_DATA_4   ROFF---tx-skip---SYSI-2935---TXN)) ;; ""
(defun (tx-skip---SYSI-2935---current-block-is-genesis)     (shift   transaction/SYST_TXN_DATA_5   ROFF---tx-skip---SYSI-2935---TXN)) ;; ""
(defun (tx-skip---SYSI-2935---current-block-isnt-genesis)   (force-bin (-  1  (tx-skip---SYSI-2935---current-block-is-genesis))))


(defun (tx-skip---SYSI-2935---sys-txn-is-nontrivial)      (force-bin (*  (tx-skip---SYSI-2935---sys-smc-has-code)
                                                                         (tx-skip---SYSI-2935---current-block-isnt-genesis))))
(defun (tx-skip---SYSI-2935---sys-txn-is-trivial)         (force-bin (-  1   (tx-skip---SYSI-2935---sys-txn-is-nontrivial))))


(defun    (tx-skip---precondition---SYSI-2935)    (force-bin (*   (-    TOTL_TXN_NUMBER    (prev    TOTL_TXN_NUMBER))
                                                                  SYSI
                                                                  TX_SKIP
                                                                  (shift    transaction/EIP_2935    ROFF---tx-skip---SYSI-2935---TXN)
                                                       )))
