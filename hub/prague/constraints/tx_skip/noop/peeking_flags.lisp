(module hub)



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The NOOP-transaction case                          ;;
;;   X.Y.Z Setting the peeking flags                        ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   tx-skip---NOOP---setting-peeking-flags
		 (:guard (tx-skip---precondition---NOOP))
		 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
		 (eq!    (+   (shift    PEEK_AT_TRANSACTION   tx-skip---NOOP---row-offset---TXN                         )
			      (shift    PEEK_AT_CONTEXT       tx-skip---NOOP---row-offset---CON---final-zero-context    ))
			 tx-skip---NOOP---NSR))


(defproperty   tx-skip---NOOP---sanity-checks
	(if-not-zero  (tx-skip---precondition---NOOP)
	       ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
	       (begin
		 (eq!    TX_SKIP                1)
		 (eq!    PEEK_AT_TRANSACTION    1)
		 )))
