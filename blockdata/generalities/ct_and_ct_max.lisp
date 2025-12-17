(module blockdata)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;  2.X CT and CT_MAX constraints  ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint  ct-and-ct-max---unconditional-setting-of-CT_MAX ()
		(eq! CT_MAX (ct-max-sum)))

(defconstraint  ct-and-ct-max---CT-vanishes-on-padding-rows-and-on-the-first-non-padding-row ()
		(if-zero    IOMF
			    (begin
			      (vanishes!         CT)
			      (vanishes!   (next CT))
			      )))

(defconstraint  ct-and-ct-max---CT-updates-before-CT_MAX-threshold
		(:guard   IOMF)
		(if-not-zero   (-  CT_MAX  CT)
			       (will-inc!   CT   1)))

(defproperty    ct-and-ct-max---CT-updates-before-CT_MAX-threshold---sanity-checks
		(if-not-zero   IOMF
			       (if-not-zero   (-  CT_MAX  CT)
					      (begin
						(eq!   (upcoming-legal-phase-transition)  0 )
						(eq!   (upcoming-phase-is-different)      0 )
						(eq!   (upcoming-phase-is-the-same)       1 )
						))))


(defconstraint  ct-and-ct-max---CT-updates-at-CT_MAX-threshold
		(:guard   IOMF)
		(if-zero   (-  CT_MAX  CT)
			   (vanishes!   (next   CT))))

(defproperty    ct-and-ct-max---CT-updates-at-CT_MAX-threshold---sanity-checks
		(if-not-zero   IOMF
			       (if-zero   (-  CT_MAX  CT)
					  (begin
					    (eq!   (upcoming-legal-phase-transition)   1 )
					    (eq!   (upcoming-phase-is-different)       1 )
					    (eq!   (upcoming-phase-is-the-same)        0 )
					    ))))

