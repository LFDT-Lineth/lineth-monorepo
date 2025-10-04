(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;   X.Y Transaction phase flags   ;;
;;   X.Y.Z Generalities            ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    phase-flags---setting-the-transaction-phase-flag-sum ()
		  (eq!    (phase-flag-sum)
			  (system-flag-sum)))

(defconstraint    phase-flags---weighted-transaction-phase-flag-sum-constancy ()
		  (hub-stamp-constancy    (phase-wght-sum)))

