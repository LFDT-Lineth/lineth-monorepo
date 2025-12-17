(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;;    X.Y.Z TOTL_TXN_NUMBER constraints    ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defcomputedcolumn  ( TOTL_TXN_NUMBER :i24 )  (+  SYSI_TXN_NUMBER
                                                  USER_TXN_NUMBER
                                                  SYSF_TXN_NUMBER))

(defconstraint    totl-txn-number-constraints---counter-constancy                         ()             (counter-constancy   CT  TOTL_TXN_NUMBER))
(defconstraint    totl-txn-number-constraints---vanishes-initially                        (:domain {0})  (vanishes!               TOTL_TXN_NUMBER)) ;; ""
(defconstraint    totl-txn-number-constraints---increments                                ()             (will-inc!               TOTL_TXN_NUMBER   (next HUB)))
(defproperty      totl-txn-number-constraints---zero-one-increments                                      (has-0-1-increments      TOTL_TXN_NUMBER))
(defconstraint    totl-txn-number-constraints---TOTL_TXN_NUMBER-is-pegged-to-txn-flag-sum ()             (if-zero                 TOTL_TXN_NUMBER
																  (eq!    (txn-flag-sum)    0) ;; BLK = 0
																  (eq!    (txn-flag-sum)    1) ;; BLK ≠ 0
																  ))

