(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                      ;;
;;    X.Y.Z Finalization constraints    ;;
;;                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    finalization-constraints (:guard (txn-flag-sum) :domain {-1}) ;; ""
                  (begin   (eq!   SYSF      1)
                           (eq!   CT_MAX   CT)))
