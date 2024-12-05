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
                ( INITIAL_BALANCE              :i128 ) ;; TODO: vastly exagerated
                ( VALUE                        :i128 ) ;; TODO: vastly exagerated

                ;; to account related
                ( TO_ADDRESS_HI                :i32  )
                ( TO_ADDRESS_LO                :i128 )
                ( REQUIRES_EVM_EXECUTION       :binary@prove ) ;; TODO: demote to debug constraint
                ( COPY_TXCD                    :binary@prove ) ;; TODO: demote to debug constraint
                ( IS_DEPLOYMENT                :binary )
                ( IS_TYPE2                     :binary )

                ;; gas related
                ( GAS_LIMIT                    :i64 )
                ( GAS_INITIALLY_AVAILABLE      :i64 )
                ( GAS_PRICE                    :i64 )
                ( PRIORITY_FEE_PER_GAS         :i64 )
                ( BASEFEE                      :i64 ) ;; TODO: vastly exagerated for Linea application

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
