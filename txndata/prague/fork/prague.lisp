(module txndata)

(defun    (first-SYSI-row)   (force-bin (*   (prev (- 1 SYSI))  SYSI)))

(defconstraint    fork-specifics---prague-specifics---SYSI-transactions ()
		  (if-not-zero    SYSI
				  (if-not-zero    HUB
						  (eq!    (+   hub/EIP_4788
						  	       hub/EIP_2935)
						          1))))

(defconstraint    fork-specifics---prague-specifics---SYSF-transactions ()
		  (if-not-zero    SYSF
				  (if-not-zero    HUB
						  (eq!    hub/NOOP    1))))

(defconstraint    fork-specifics---prague-specifics---transaction-order---4788-and-forward ()
		  (if-not-zero    (first-SYSI-row)
				  (begin
				    (eq!          hub/EIP_4788                     1)
				    (eq!  (shift  hub/EIP_2935  nROWS___EIP_4788)  1)
				    )))


(defproperty      fork-specifics---prague-specifics---transaction-order---sanity-checks ()
		  (if-not-zero    (first-SYSI-row)
				  (begin
				  (eq!          HUB                     1)
				  (eq!  (shift  HUB  nROWS___EIP_4788)  1)
				  )))

(defconstraint    fork-specifics---prague-specifics---transaction-order---2935-and-forward ()
		  (if-not-zero    SYSI
		                  (if-not-zero    HUB
				                  (if-not-zero    hub/EIP_2935
						                  (eq!   (shift   (+ USER SYSF)     nROWS___EIP_2935)   1)
								  ))))
