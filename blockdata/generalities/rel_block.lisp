(module blockdata)


;;;;;;;;;;;;;;;;;;;;;
;;                 ;;
;;  2.X REL_BLOCK  ;;
;;                 ;;
;;;;;;;;;;;;;;;;;;;;;

(defproperty      rel-block---initial-vanishing (:domain {0}) ;; ""
		  (vanishes!   REL_BLOCK))

(defproperty      rel-block---has-0-1-increments ()
		  (has-0-1-increments   REL_BLOCK))

(defconstraint    rel-block---precise-increments ()
		  (will-inc!   REL_BLOCK
			       (about-to-start-new-block)))

(defconstraint    rel-block---pegging-REL_BLOCK-agains-IOMF ()
		  (if-zero   REL_BLOCK
			     (eq!   IOMF   0)
			     (eq!   IOMF   1)
			     ))

