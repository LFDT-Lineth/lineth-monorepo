(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    X.Y.Z Prelude    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;


(defconst
  ROFF___SYSI___HUB_ROW   0
  ROFF___SYSI___CMP_ROW   1

  nROWS___SYSI___NOOP       2
  nROWS___SYSI___EIP_4788   4
  nROWS___SYSI___EIP_2935   4
  )


(defun   (first-row-of-SYSI-transaction)   (*   (-   TOTL_TXN_NUMBER   (prev   TOTL_TXN_NUMBER))   SYSI))

(defconstraint   SYSI-prelude-constraints---setting-the-first-few-perspectives   (:guard (first-row-of-SYSI-transaction))
		 (eq!
		   (+  ( shift  HUB    ROFF___SYSI___HUB_ROW )
		       ( shift  CMPTN  ROFF___SYSI___CMP_ROW ))
		   2))

(defconstraint   SYSI-prelude-constraints---imposing-a-SYSI-transaction-scenario   (:guard (first-row-of-SYSI-transaction))
		 (eq!
		   (+  ( shift   hub/NOOP       ROFF___SYSI___HUB_ROW )
		       ( shift   hub/EIP_4788   ROFF___SYSI___HUB_ROW )
		       ( shift   hub/EIP_2935   ROFF___SYSI___HUB_ROW ))
		   1))

(defun   (ct-max-SYSI-sum)   (+  (*  (-  nROWS___SYSI___NOOP      1)  ( shift  hub/NOOP      ROFF___SYSI___HUB_ROW ))
				 (*  (-  nROWS___SYSI___EIP_4788  1)  ( shift  hub/EIP_4788  ROFF___SYSI___HUB_ROW ))
				 (*  (-  nROWS___SYSI___EIP_2935  1)  ( shift  hub/EIP_2935  ROFF___SYSI___HUB_ROW ))))

(defconstraint   SYSI-prelude-constraints---setting-CT_MAX                         (:guard (first-row-of-SYSI-transaction))
		 (eq!   CT_MAX   (ct-max-SYSI-sum)))

;; constraint to be removed for Prague
(defconstraint   SYSI-prelude-constraints---disallowing-EIP-2935-in-Cancun         (:guard (first-row-of-SYSI-transaction))
		 (vanishes!   ( shift  hub/EIP_2935  ROFF___SYSI___HUB_ROW )))
