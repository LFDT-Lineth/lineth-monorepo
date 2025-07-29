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
