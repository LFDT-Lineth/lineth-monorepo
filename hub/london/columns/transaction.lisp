(module hub)

(defperspective transaction

	;; selector
	PEEK_AT_TRANSACTION

	;; transaction-row columns
        (
                ;; from account related
                ( FROM_ADDRESS_HI              :i32  )
                ( FROM_ADDRESS_LO              :i128 )
                ( NONCE                        :i64  )
                ( INITIAL_BALANCE              :i128 )
                ( VALUE                        :i128 )

                ;; to account related
                ( TO_ADDRESS_HI                :i32  )
                ( TO_ADDRESS_LO                :i128 )
                ( REQUIRES_EVM_EXECUTION       :binary )
                ( COPY_TXCD                    :binary )
                ( IS_DEPLOYMENT                :binary )
                ( IS_TYPE2                     :binary )

                ;; gas related
                ( GAS_LIMIT                    :i64 )
                ( GAS_INITIALLY_AVAILABLE      :i64 )
                ( GAS_PRICE                    :i64 )
                ( PRIORITY_FEE_PER_GAS         :i64 )
                ( BASEFEE                      :i64 ) ;; in Linea London this is hard-coded to 7 ... but in the reference tests this may be much larger

                ;; call data or init code
                ( CALL_DATA_SIZE               :i32 )
                ( INIT_CODE_SIZE               :i32 )

                ;; end of transaction predictions
                ( STATUS_CODE                  :binary)
                ( GAS_LEFTOVER                 :i64 )
                ( REFUND_COUNTER_INFINITY      :i64 )
                ( REFUND_EFFECTIVE             :i64 )

                ;; coinbase related
                ( COINBASE_ADDRESS_HI          :i32  )
                ( COINBASE_ADDRESS_LO          :i128 )
        )
)
