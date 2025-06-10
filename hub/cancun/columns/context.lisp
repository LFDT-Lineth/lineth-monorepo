(module hub)

(defperspective context

		;; selector
		PEEK_AT_CONTEXT

		;; context-row columns
		(
		 ;; (immutable) context data
		 ( CONTEXT_NUMBER                    :i32          ) ;; rmk: deduced from the HUB_STAMP; could likely be reduced to :i24 (i.e. ~ 16M); though: limitless prover ...
		 ( CALL_STACK_DEPTH                  :i11          ) ;; in the range [0 .. 1024] (inclusive)
		 ( IS_ROOT                           :binary@prove ) ;; rmk: set at the creation of the context explicitly to 0 or 1; imposed to be context-constant; the @prove is therefore redundant; we keep it for now as a safet;
		 ( IS_STATIC                         :binary@prove ) ;; rmk: same remark 

		 ;; (immutable) account
		 ( ACCOUNT_ADDRESS_HI                :i32  )
		 ( ACCOUNT_ADDRESS_LO                :i128 )
		 ( ACCOUNT_DEPLOYMENT_NUMBER         :i32  )

		 ;; (immutable) account whose bytecode is being executed
		 ( BYTE_CODE_ADDRESS_HI              :i32  )
		 ( BYTE_CODE_ADDRESS_LO              :i128 )
		 ( BYTE_CODE_DEPLOYMENT_NUMBER       :i32  )
		 ( BYTE_CODE_DEPLOYMENT_STATUS       :i32  )
		 ( BYTE_CODE_CODE_FRAGMENT_INDEX     :i32  )

		 ;; (immutable) caller account
		 ( CALL_DATA_CONTEXT_NUMBER          :i32  )
		 ( CALLER_ADDRESS_HI                 :i32  )
		 ( CALLER_ADDRESS_LO                 :i128 )
		 ( CALL_VALUE                        :i128 )

		 ;; (immutable) parameters set at CALL
		 ( CALL_DATA_OFFSET                  :i32  )
		 ( CALL_DATA_SIZE                    :i32  )
		 ( RETURN_AT_OFFSET                  :i32  )
		 ( RETURN_AT_CAPACITY                :i32  )

		 ;; (mutable) parameters set when execution resumes after a CALL / CREATE
		 ( UPDATE                            :binary@prove )
		 ( RETURN_DATA_OFFSET                :i32 )
		 ( RETURN_DATA_SIZE                  :i32 )
		 ( RETURN_DATA_CONTEXT_NUMBER        :i32 )
		 ))
