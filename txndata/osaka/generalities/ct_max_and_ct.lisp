(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                       ;;
;;    X.Y.Z CT_MAX and CT constraints    ;;
;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    CT_MAX-and-CT-constraints---CT_MAX-counter-constancy  ()
		  (counter-constancy    CT   CT_MAX))

(defconstraint    CT_MAX-and-CT-constraints---automatic-vanishing ()
		  (if-zero   (txn-flag-sum)
			     (begin
			       (vanishes!  CT_MAX )
			       (vanishes!  CT     ))))

(defconstraint    CT_MAX-and-CT-constraints---CT-update-constraints ()
		  (if-eq-else    CT   CT_MAX
				 (eq!  (next   CT)   0)        ;; CT = CT_MAX
				 (eq!  (next   CT)  (+ CT 1))  ;; CT = CT_MAX
				 ))

