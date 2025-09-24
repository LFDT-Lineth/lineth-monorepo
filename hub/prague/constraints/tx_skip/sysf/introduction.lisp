(module hub)



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The SYSF-transaction case                          ;;
;;   X.Y.Z Generalities                                     ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   tx-skip---SYSF---generalities ()
		 (if-not-zero   (tx-skip---precondition---SYSF)
				(begin
				  (eq!    TX_SKIP             1)
				  (eq!    transaction/NOOP    1))))

