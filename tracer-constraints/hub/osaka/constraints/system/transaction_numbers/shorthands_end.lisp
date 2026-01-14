(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                          ;;
;;   X.Y The XXX_TXN_NUMBER columns         ;;
;;   X.Y.Z Shorthands for transaction end   ;;
;;                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun   (system-txn-numbers---sysi-txn-end)   (force-bin  (*    SYSI         TX_SKIP    CON)))
(defun   (system-txn-numbers---sysf-txn-end)   (force-bin  (*    SYSF         TX_SKIP    CON)))
(defun   (system-txn-numbers---user-txn-end)   (force-bin  (*    USER    (+   TX_SKIP
								    TX_FINL)   CON)))

(defun    (system-txn-numbers---txn-end)    (force-bin (+  (system-txn-numbers---sysi-txn-end)
						  						 		   (system-txn-numbers---user-txn-end)
						  						 		   (system-txn-numbers---sysf-txn-end)
						  						  			)))

