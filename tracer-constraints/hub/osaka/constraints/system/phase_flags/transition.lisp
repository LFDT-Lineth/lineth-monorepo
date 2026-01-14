(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;   X.Y Transaction phase flags   ;;
;;   X.Y.Z Transitions             ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

; TX_SKIP case
;-------------

(defconstraint      phase-flags---legal-transitions-out-of-TX_SKIP ()
		    (if-not-zero    TX_SKIP
				    (eq!  (next  (+  TX_SKIP
						     TX_WARM
						     TX_INIT))
					  1)))

(defproperty    phase-flags---legal-transitions-out-of-TX_SKIP---formal-consequences
	(if-not-zero    TX_SKIP
		(if-zero    CON
			    ;; CON = 0
			    (eq!  (next  TX_SKIP)      1)
			    ;; CON = 1
			    (eq!  (next  (+  TX_SKIP
					     TX_WARM
					     TX_INIT)) 1))))


; TX_WARM case
;-------------

(defconstraint      phase-flags---legal-transitions-out-of-TX_WARM ()
		    (if-not-zero    TX_WARM
				    (begin
				      (eq!  (next  (+  TX_WARM  TX_INIT)) 1)
				      (eq!  (next               TX_INIT)  (next  TXN))
				      )))

(defproperty      phase-flags---legal-transitions-out-of-TX_WARM---formal-consequences
		  (if-not-zero    TX_WARM
				  (if-zero     (next  TXN)
					       (eq!  (next  TX_WARM)  1)
					       (eq!  (next  TX_INIT)  1))
				  ))


; TX_INIT case
;-------------

(defconstraint      phase-flags---legal-transitions-out-of-TX_INIT ()
		    (if-not-zero    TX_INIT
				    (begin
				      (eq!  (next  (+  TX_INIT  TX_EXEC))  1)
				      (eq!  (next               TX_EXEC)   CON)
				      )))


; TX_EXEC case
;-------------

(defconstraint      phase-flags---legal-transitions-out-of-TX_EXEC ()
		    (if-not-zero    TX_EXEC
				    (begin
				      (eq!  (next  (+  TX_EXEC  TX_FINL))  1)
				      (if-not       (will-remain-constant!    HUB_STAMP)
						    (if-not-zero    CN_NEW
								    ;; CN_NEW ≠ 0
								    (eq!    (next    TX_EXEC)    1)
								    ;; CN_NEW = 0
								    (eq!    (next    TX_FINL)    1)))
				      )))

(defproperty    phase-flags---legal-transitions-out-of-TX_EXEC---formal-consequences
			(if-not-zero    TX_EXEC
					(if-not       (will-inc!  HUB_STAMP  1)
			    		  (eq!    (next    TX_EXEC)    1)
			      		  )))


; TX_FINL case
;-------------

(defconstraint      phase-flags---legal-transitions-out-of-TX_FINL ()
		    (if-not-zero    TX_FINL
				    (begin
				      (eq!  (next  (+  TX_FINL
						       TX_SKIP
						       TX_WARM
						       TX_INIT))   1)
				      (eq!  (next  (+  TX_SKIP
						       TX_WARM
						       TX_INIT))   CON)
				      )))

