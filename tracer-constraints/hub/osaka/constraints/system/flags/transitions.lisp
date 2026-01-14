(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;   X.Y System flags    ;;
;;   X.Y.Z Transitions   ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    system-flag-generalities---transitions ()
		  (begin
		    ;; (if-not-zero    SYSI    (eq!    (next    (+ SYSI    USER        ))    1)) ;; <= empty blocks disallowed
		    (if-not-zero    USER    (eq!    (next    (+         USER    SYSF))    1))
		    (if-not-zero    SYSF    (eq!    (next    (+ SYSI            SYSF))    1))
		    ))


(defproperty    system-flag-generalities---transitions---allow-for-empty-blocks
		(if-not-zero    SYSI    (eq!    (next    (+ SYSI    USER    SYSF))    1)) ;; <= empty blocks OK
		)
