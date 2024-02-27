(module hub_v2)

(defperspective storage
	
	;; selector
	PEEK_AT_STORAGE
	
	;; storage-row columns
	(
		ADDRESS_HI
		ADDRESS_LO
		DEPLOYMENT_NUMBER
		STORAGE_KEY_HI
		STORAGE_KEY_LO
		VAL_ORIG_HI
		VAL_ORIG_LO
		VAL_CURR_HI
		VAL_CURR_LO
		VAL_NEXT_HI
		VAL_NEXT_LO
		( WARM                          :binary )
		( WARM_NEW                      :binary )

		( VAL_ORIG_IS_ZERO              :binary )
		( VAL_CURR_IS_ORIG              :binary )
		( VAL_CURR_IS_ZERO              :binary )
		( VAL_NEXT_IS_CURR              :binary )
		( VAL_NEXT_IS_ZERO              :binary )
		( VAL_NEXT_IS_ORIG              :binary )
		( VAL_CURR_CHANGES              :binary )
	))
