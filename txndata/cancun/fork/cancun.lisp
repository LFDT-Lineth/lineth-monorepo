(module txndata)

(defun    (first-SYSI-row)   (force-bin (*   (prev (- 1 SYSI))  SYSI)))


(defconstraint    fork-specifics---cancun-specifics---SYSI-transactions ()
		  (if-not-zero    SYSI
				  (if-not-zero    HUB
						  (eq!    hub/EIP_4788    1))))

(defconstraint    fork-specifics---cancun-specifics---SYSF-transactions ()
		  (if-not-zero    SYSF
				  (if-not-zero    HUB
						  (eq!    hub/NOOP    1))))

(defconstraint    fork-specifics---cancun-specifics---transaction-order ()
		  (if-not-zero    (first-SYSI-row)
				  (begin
				    (eq!    hub/EIP_4788    1)
				    (eq!   (shift   (+ USER SYSF)   nROWS___EIP_4788)   1)
				    )))

(defproperty      fork-specifics---cancun-specifics---transaction-order---sanity-checks ()
		  (if-not-zero    (first-SYSI-row)
				  (eq!  HUB  1)))
