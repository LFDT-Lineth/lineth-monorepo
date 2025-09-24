(module hub)

(defperspective transient

	;; selector
	PEEK_AT_TRANSIENT

	;; storage-row columns
	(
		( ADDRESS_HI                      :i32  )
		( ADDRESS_LO                      :i128 )
		( STORAGE_KEY_HI                  :i128 )
		( STORAGE_KEY_LO                  :i128 )
		( VALUE_CURR_HI                   :i128 )
		( VALUE_CURR_LO                   :i128 )
		( VALUE_NEXT_HI                   :i128 )
		( VALUE_NEXT_LO                   :i128 )
	))

