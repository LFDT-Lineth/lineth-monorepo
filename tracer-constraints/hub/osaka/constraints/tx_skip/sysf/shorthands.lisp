(module hub)



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The SYSF-transaction case                          ;;
;;   X.Y.Z Shorthands                                       ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (tx-skip---precondition---SYSF)    (*   (-    TOTL_TXN_NUMBER    (prev    TOTL_TXN_NUMBER))
						  SYSF
						  ))

(defconst
  ROFF___TX_SKIP___NOOP___TRANSACTION_ROW  0
  ROFF___TX_SKIP___NOOP___ZERO_CONTEXT_ROW 1
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  NSR___TX_SKIP___NOOP                     2
  )
