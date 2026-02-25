(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                    ;;
;;   X.Y The XXX_TXN_NUMBER columns   ;;
;;   X.Y.Z Housekeeping               ;;
;;                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconst    ZERO_CONTEXT    0)


(defconstraint    system-txn-numbers---housekeeping---what-we-call-the-beginning-is-often-the-end    (:guard    TOTL_TXN_NUMBER)
                  (will-inc!    TOTL_TXN_NUMBER    (system-txn-numbers---txn-end)))

(defproperty      system-txn-numbers---housekeeping---and-to-make-an-end-is-to-make-a-beginning
                  (did-inc!    TOTL_TXN_NUMBER    (system-txn-numbers---txn-start)))

(defconstraint    system-txn-numbers---housekeeping---the-end-is-not-where-we-start-from    ()
                  (vanishes!    (*    (system-txn-numbers---txn-start)
                                      (system-txn-numbers---txn-end))))

(defconstraint    system-txn-numbers---housekeeping---the-final-context-row-of-every-transaction    ()
                  (if-not-zero    (system-txn-numbers---txn-end)
                                  (read-context-data    0
                                                        ZERO_CONTEXT)))

(defconstraint    system-txn-numbers---housekeeping---the-final-row-of-every-trace    (:domain {-1}) ;; ""
                  (begin
                    (eq!    SYSF    1)
                    (eq!    (system-txn-numbers---txn-end)    1)))
