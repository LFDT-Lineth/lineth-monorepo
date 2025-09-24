(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                            ;;
;;   X.Y The XXX_TXN_NUMBER columns           ;;
;;   X.Y.Z Shorthands for transaction start   ;;
;;                                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (system-txn-numbers---sysi-txn-start)    (*    SYSI    TX_SKIP    TXN))
(defun    (system-txn-numbers---sysf-txn-start)    (*    SYSF    TX_SKIP    TXN))
(defun    (system-txn-numbers---user-txn-start)    (*    USER    (+    (*                                     TX_SKIP    TXN)
								       (*    (-    1    (prev    TX_WARM))    TX_WARM)
								       (*    (-    1    (prev    TX_WARM))
									     (-    1    (prev    TX_INIT))    TX_INIT))))
(defun    (system-txn-numbers---txn-start)         (+    (system-txn-numbers---sysi-txn-start)
							 (system-txn-numbers---user-txn-start)
							 (system-txn-numbers---sysf-txn-start)
							 ))
