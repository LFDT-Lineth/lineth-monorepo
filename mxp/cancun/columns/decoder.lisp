(module mxp)

(defperspective decoder
		;; selector
		DECODER
		;; instruction decoded columns
		(
		 ( INST                 :byte   )
		 ( IS_MSIZE             :binary )
		 ( IS_RETURN            :binary )
		 ( IS_MCOPY             :binary )
		 ( IS_FIXED_SIZE_32     :binary )
		 ( IS_FIXED_SIZE_1      :binary )
		 ( IS_SINGLE_MAX_OFFSET :binary )
		 ( IS_DOUBLE_MAX_OFFSET :binary )
		 ( IS_WORD_PRICING      :binary )
		 ( IS_BYTE_PRICING      :binary )
		 ( G_WORD               :byte   )
		 ( G_BYTE               :byte   )
		 )
		)
