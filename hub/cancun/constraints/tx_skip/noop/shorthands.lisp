(module hub)



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The NOOP-transaction case                          ;;
;;   X.Y.Z Shorthands                                       ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defconst
  tx-skip---NOOP---row-offset---TXN                        0
  tx-skip---NOOP---row-offset---CON---final-zero-context   1
  tx-skip---NOOP---NSR                                     2
  )


(defun    (tx-skip---precondition---NOOP)    (*   (-    TOTL_TXN_NUMBER    (prev    TOTL_TXN_NUMBER))
						  (+    SYSI    SYSF)
						  transaction/NOOP
						  ))

