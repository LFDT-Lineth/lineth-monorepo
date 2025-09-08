(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;    X.Y.Y CMPTN-view columns    ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defperspective computation

	;; selector
	CMPTN

	;; CMPTN view columns
	(
         ( EUC_FLAG      :binary@prove )
         ( WCP_FLAG      :binary@prove )
         ( ARG_1_LO      :i128         )
         ( ARG_2_LO      :i128         )
         ( INST          :i8           )
         ( WCP_RES       :binary@prove ) ;; the @prove isn't strictly speaking necessary
         ( EUC_QUOTIENT  :i64          )
         ( EUC_REMAINDER :i64          )
         ))

