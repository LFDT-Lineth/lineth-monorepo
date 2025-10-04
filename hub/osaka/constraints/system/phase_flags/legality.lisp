(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;   X.Y Transaction phase flags   ;;
;;   X.Y.Z Legality                ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    phase-flags---legal-processing-phases---the-SYSI-case ()
		  (if-not-zero    SYSI
				  (eq!    1    TX_SKIP)))

(defproperty    phase-flags---legal-processing-phases---the-USER-case
		(if-not-zero    USER
				(eq!    1    (+    TX_SKIP
						   TX_WARM
						   TX_INIT
						   TX_EXEC
						   TX_FINL))))

(defconstraint    phase-flags---legal-processing-phases---the-SYSF-case ()
		  (if-not-zero    SYSF
				  (eq!    1    TX_SKIP)))
