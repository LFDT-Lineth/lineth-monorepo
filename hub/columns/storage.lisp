(module hub)

(defperspective storage
	
	;; selector
	PEEK_AT_STORAGE
	
	;; storage-row columns
	(
		ADDRESS_HI
		ADDRESS_LO
		DEPLOYMENT_NUMBER
		DEPLOYMENT_NUMBER_INFTY
		STORAGE_KEY_HI
		STORAGE_KEY_LO
		VALUE_ORIG_HI
		VALUE_ORIG_LO
		VALUE_CURR_HI
		VALUE_CURR_LO
		VALUE_NEXT_HI
		VALUE_NEXT_LO

		( WARMTH                        :binary@prove )
		( WARMTH_NEW                    :binary@prove )

		( VALUE_ORIG_IS_ZERO              :binary ) ;; @prove not required for any of these since set by hand
		( VALUE_CURR_IS_ORIG              :binary )
		( VALUE_CURR_IS_ZERO              :binary )
		( VALUE_NEXT_IS_CURR              :binary )
		( VALUE_NEXT_IS_ZERO              :binary )
		( VALUE_NEXT_IS_ORIG              :binary )
		( VALUE_CURR_CHANGES              :binary )
	))
