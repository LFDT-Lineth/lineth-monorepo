(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                      ;;
;;    X.Y.Z txn_flag_sum constraints    ;;
;;                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (txn-flag-sum)    (force-bin  (+  SYSI  USER  SYSF)))

(defconstraint    txn-flag-sum-constraints---binary-constraint ()
                  (is-binary    (txn-flag-sum)))

(defconstraint    txn-flag-sum-constraints---initially-zero    (:domain {0}) ;; ""
                  (vanishes!    (txn-flag-sum)))

(defconstraint    txn-flag-sum-constraints---monotonicity ()
                  (if-not-zero    (txn-flag-sum)
                                  (will-eq!    (txn-flag-sum)    1)))
