(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;   X.Y System flags              ;;
;;   X.Y.Z Transaction constancy   ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    system-flag-generalities---transaction-constancy ()
		  (transaction-constancy    (system-wght-sum)))
