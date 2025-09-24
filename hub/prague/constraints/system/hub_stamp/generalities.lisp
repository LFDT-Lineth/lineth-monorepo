(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;   X.Y The HUB_STAMP column   ;;
;;   X.Y.Z Generalities         ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    system---hub-stamp---initialization (:domain {0}) ;; ""
		  (vanishes!    HUB_STAMP))

(defconstraint    system---hub-stamp---0-1-increments ()
		  (has-0-1-increments    HUB_STAMP))

(defconstraint    system---hub-stamp---pegging-to-system-flag-sum ()
		  (if-not-zero    HUB_STAMP
				  (eq!    (system-flag-sum)   1)
				  (eq!    (system-flag-sum)   0)))

