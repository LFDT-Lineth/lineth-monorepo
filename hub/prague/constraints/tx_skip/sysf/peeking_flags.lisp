(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The SYSF-transaction case                          ;;
;;   X.Y.Z Setting the peeking flags                        ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    tx-skip---SYSF---setting-the-peeking-flags ()
		  (if-not-zero   (tx-skip---precondition---SYSF)
				 (eq!    (+   (shift   TXN   ROFF___TX_SKIP___NOOP___TRANSACTION_ROW  )
					      (shift   CON   ROFF___TX_SKIP___NOOP___ZERO_CONTEXT_ROW )
					      )
					 NSR___TX_SKIP___NOOP
					 )))

