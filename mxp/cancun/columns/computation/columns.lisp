(module mxp)


(defperspective computation
		;; selector
		COMPUTATION
		;; computation columns
		(
		 ( WCP_FLAG :binary@prove )
		 ( EUC_FLAG :binary@prove )
		 ( EXO_INST :byte         )
		 ( ARG_1_HI :i128         )
		 ( ARG_1_LO :i128         )
		 ( ARG_2_HI :i128         )
		 ( ARG_2_LO :i128         )
		 ( RES_A    :i32          )
		 ( RES_B    :i32          )
		 )
		)
