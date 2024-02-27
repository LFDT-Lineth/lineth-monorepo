(module hub_v2)

(defperspective context

		;; selector
		PEEK_AT_CONTEXT

		;; context-row columns
		(
		 ;; (immutable) context data
		 CONTEXT_NUMBER
		 CALL_STACK_DEPTH
		 ( IS_ROOT                      :binary@prove )   ;; binaryness should be demoted to a debug cosntraint
		 ( IS_STATIC                    :binary@prove )   ;; binaryness should be demoted to a debug cosntraint

		 ;; (immutable) account
		 ACCOUNT_ADDRESS_HI
		 ACCOUNT_ADDRESS_LO
		 ACCOUNT_DEPLOYMENT_NUMBER

		 ;; (immutable) account whose bytecode is being executed
		 BYTE_CODE_ADDRESS_HI
		 BYTE_CODE_ADDRESS_LO
		 BYTE_CODE_DEPLOYMENT_NUMBER
		 BYTE_CODE_DEPLOYMENT_STATUS
		 BYTE_CODE_CODE_FRAGMENT_INDEX

		 ;; (immutable) caller account
		 CALLER_CONTEXT_NUMBER
		 CALLER_ADDRESS_HI
		 CALLER_ADDRESS_LO
		 CALL_VALUE

		 ;; (immutable) parameters set at CALL
		 CALL_DATA_OFFSET
		 CALL_DATA_SIZE
		 RETURN_AT_OFFSET
		 RETURN_AT_CAPACITY

		 ;; (mutable) parameters set when execution resumes after a CALL / CREATE
		 ( UPDATE                       :binary@prove )
		 RETURN_DATA_OFFSET
		 RETURN_DATA_SIZE
		 RETURNER_CONTEXT_NUMBER
		 ))
