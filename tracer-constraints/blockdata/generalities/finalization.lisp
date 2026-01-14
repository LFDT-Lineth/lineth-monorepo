(module blockdata)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;  2.X Finalization constraints  ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   finalization-constraints (:domain {-1}) ;; ""
		 (begin
		   (eq!   IS_BL    1  )
		   (eq!   CT_MAX   CT )
		   ))
