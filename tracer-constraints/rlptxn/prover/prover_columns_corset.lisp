(module rlptxn)



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;   Constraints verification   ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defcolumns
  ( prover___USER_TXN_NUMBER_MAX :i16 )
  )

(defalias
  USRMAX   prover___USER_TXN_NUMBER_MAX
  )



(defconstraint  prover-column-constraints---USER_TXN_NUMBER_MAX---constancy ()
		(if-not-zero   USER_TXN_NUMBER
			       (will-remain-constant!   USRMAX)))

(defconstraint  prover-column-constraints---USER_TXN_NUMBER_MAX---finalization
		(:domain {-1}) ;; ""
		(eq!   USRMAX   USER_TXN_NUMBER))



;; TOTL|USER|SYSF|SYSI|prover
