(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;   X.Y System flags    ;;
;;   X.Y.Z Transitions   ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    system-flag-generalities---transitions ()
		  (begin
		    (if-not-zero    SYSI    (eq!    (next    (+ SYSI    USER        ))    1))
		    (if-not-zero    USER    (eq!    (next    (+         USER    SYSF))    1))
		    (if-not-zero    SYSF    (eq!    (next    (+ SYSI            SYSF))    1))
		    ))
