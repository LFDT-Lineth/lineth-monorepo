(module hub_v2)

(defperspective transaction

	;; selector
	PEEK_AT_TRANSACTION

	;; transaction-row columns
        (
                BATCH_NUM

                ;; from account related
                FROM_ADDRESS_HI
                FROM_ADDRESS_LO
                NONCE
                INITIAL_BALANCE
                VALUE

                ;; to account related
                TO_ADDRESS_HI
                TO_ADDRESS_LO
                (REQUIRES_EVM_EXECUTION       :binary@prove ) ;; TODO: demote to debug constraint
                (COPY_TXCD_AT_INITIALIZATION  :binary@prove ) ;; TODO: demote to debug constraint
                (IS_DEPLOYMENT                :binary )
                (IS_TYPE2                     :binary )

                ;; gas related
                GAS_LIMIT
                INITIAL_GAS
                GAS_PRICE
                BASEFEE

                ;; call data or init code
                CALL_DATA_SIZE
                INIT_CODE_SIZE

                ;; end of transaction predictions
                (STATUS_CODE                  :binary)
                LEFTOVER_GAS
                REFUND_COUNTER_INFINITY
                REFUND_AMOUNT
                
                ;; coinbase related
                COINBASE_ADDRESS_HI
                COINBASE_ADDRESS_LO
        )
)
