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
  ROFF---tx-skip---SYSI-4788---TXN                                        0
  ROFF---tx-skip---SYSI-4788---ACC---loading-the-beacon-root-account      1
  ROFF---tx-skip---SYSI-4788---STO---storing-the-time-stamp               2
  ROFF---tx-skip---SYSI-4788---STO---storing-the-beacon-root              3
  ROFF---tx-skip---SYSI-4788---CON---final-zero-context---nontrivial-case 4
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  ROFF---tx-skip---SYSI-4788---CON---final-zero-context---trivial-case    2
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  NSR---tx-skip---SYSI-4788---unconditional                                     2
  NSR---tx-skip---SYSI-4788---trivial-case                                      3
  NSR---tx-skip---SYSI-4788---nontrivial-case                                   5
  )

(defun (tx-skip---SYSI-4788---sys-smc-has-code)             (shift   account/HAS_CODE              ROFF---tx-skip---SYSI-4788---ACC---loading-the-beacon-root-account)) ;; ""
(defun (tx-skip---SYSI-4788---timestamp)                    (shift   transaction/SYST_TXN_DATA_1   ROFF---tx-skip---SYSI-4788---TXN)) ;; ""
(defun (tx-skip---SYSI-4788---timestamp-mod-8191)           (shift   transaction/SYST_TXN_DATA_2   ROFF---tx-skip---SYSI-4788---TXN)) ;; ""
(defun (tx-skip---SYSI-4788---beacon-root-hi)               (shift   transaction/SYST_TXN_DATA_3   ROFF---tx-skip---SYSI-4788---TXN)) ;; ""
(defun (tx-skip---SYSI-4788---beacon-root-lo)               (shift   transaction/SYST_TXN_DATA_4   ROFF---tx-skip---SYSI-4788---TXN)) ;; ""
(defun (tx-skip---SYSI-4788---current-block-is-genesis)     (shift   transaction/SYST_TXN_DATA_5   ROFF---tx-skip---SYSI-4788---TXN)) ;; ""
(defun (tx-skip---SYSI-4788---current-block-isnt-genesis)   (force-bin (-  1  (tx-skip---SYSI-4788---current-block-is-genesis))))


(defun (tx-skip---SYSI-4788---sys-txn-is-nontrivial)      (force-bin (*  (tx-skip---SYSI-4788---sys-smc-has-code)
                                                                         (tx-skip---SYSI-4788---current-block-isnt-genesis))))
(defun (tx-skip---SYSI-4788---sys-txn-is-trivial)         (force-bin (-  1   (tx-skip---SYSI-4788---sys-txn-is-nontrivial))))


(defun    (tx-skip---precondition---SYSI-4788)    (force-bin (*   (-  TOTL_TXN_NUMBER  (prev  TOTL_TXN_NUMBER))
                                                                  SYSI
                                                                  TX_SKIP
                                                                  (shift    transaction/EIP_4788    ROFF---tx-skip---SYSI-4788---TXN)
                                                       )))
