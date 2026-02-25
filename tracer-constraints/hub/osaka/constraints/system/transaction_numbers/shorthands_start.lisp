(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                            ;;
;;   X.Y The XXX_TXN_NUMBER columns           ;;
;;   X.Y.Z Shorthands for transaction start   ;;
;;                                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (system-txn-numbers---sysi-txn-start)    (*    SYSI    TX_SKIP    TXN))
(defun    (system-txn-numbers---sysf-txn-start)    (*    SYSF    TX_SKIP    TXN))

(defun    (system-txn-numbers---user-txn-start)    (*    USER    (+    (system-txn-numbers---user-txn-start---by-authorization )
                                                                       (system-txn-numbers---user-txn-start---by-prewarming    )
                                                                       (system-txn-numbers---user-txn-start---by-tx-init       )
                                                                       (system-txn-numbers---user-txn-start---by-tx-skip       )
                                                                       )))

(defun    (system-txn-numbers---user-txn-start---by-authorization)   (*  (prev  (-  1  TX_AUTH          ) )  TX_AUTH                      ))
(defun    (system-txn-numbers---user-txn-start---by-prewarming)      (*  (prev  (-  1  TX_AUTH  TX_WARM ) )  TX_WARM                      ))
(defun    (system-txn-numbers---user-txn-start---by-tx-init)         (*  (prev  (-  1  TX_AUTH  TX_WARM ) )  TX_INIT  PEEK_AT_TRANSACTION ))
(defun    (system-txn-numbers---user-txn-start---by-tx-skip)         (*  (prev  (-  1  TX_AUTH          ) )  TX_SKIP  PEEK_AT_TRANSACTION ))

(defun    (system-txn-numbers---txn-start)         (+    (system-txn-numbers---sysi-txn-start)
                                                         (system-txn-numbers---user-txn-start)
                                                         (system-txn-numbers---sysf-txn-start)
                                                         ))
