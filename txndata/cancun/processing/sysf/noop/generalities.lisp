(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                      ;;
;;    X. SYSF transaction processing    ;;
;;    X.Y Generalities                  ;;
;;                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defun    (first-row-of-SYSF-transaction)    (force-bin    (*   (- TOTL_TXN_NUMBER (prev TOTL_TXN_NUMBER))   SYSF)))


(defconstraint   SYSF-prelude-constraints---setting-the-first-few-perspectives
		 (:guard (first-row-of-SYSF-transaction))
		 (eq!
		   (+  ( shift  HUB    ROFF___SYSF___HUB_ROW )
		       ( shift  CMPTN  ROFF___SYSF___CMP_ROW ))
		   2))

(defconstraint   SYSF-transaction---generalities---SYSF-transactions-are-necessarily-NOOPs
		 (:guard (first-row-of-SYSF-transaction))
		 (eq!   (shift   hub/NOOP   ROFF___SYSF___HUB_ROW)   1))

(defun   (ct-max-SYSF-sum)   (*  (-  nROWS___SYSF___NOOP      1)  ( shift  hub/NOOP      ROFF___SYSF___HUB_ROW )))

(defconstraint   SYSF-prelude-constraints---setting-CT_MAX
		 (:guard (first-row-of-SYSF-transaction))
		 (eq!   CT_MAX   (ct-max-SYSF-sum)))
