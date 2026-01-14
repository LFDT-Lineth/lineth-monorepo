(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                        ;;
;;   X.Y System flags     ;;
;;   X.Y.Z Generalities   ;;
;;                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defproperty      system-flag-generalities---binary
		  (is-binary    (system-flag-sum)))

(defconstraint    system-flag-generalities---initialization (:domain {0}) ;; ""
		  (vanishes!    (system-flag-sum)))

(defconstraint    system-flag-generalities---monotonicity ()
		  (if-not-zero    (system-flag-sum)
				  (eq!    (next    (system-flag-sum))    1)))

(defconstraint    system-flag-generalities---pegging-to-block-number ()
		  (if-not-zero    BLK_NUMBER
				  (eq!    (system-flag-sum)   1)
				  (eq!    (system-flag-sum)   0)))

