(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    X.Y.Y HUB-view columns    ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defperspective hub

	;; selector
	HUB

	;; HUB view columns
        (
                ;; block data from BTC module
                ( btc_BLOCK_NUMBER           :i32          )
                ( btc_BLOCK_GAS_LIMIT        :i32          )
                ( btc_BASEFEE                :i8           )
                ( btc_TIMESTAMP              :i64          )
                ( btc_COINBASE_ADDRESS_HI    :i32          )
                ( btc_COINBASE_ADDRESS_LO    :i128         )
		;;
                ( TO_ADDRESS_HI              :i32          )
                ( TO_ADDRESS_LO              :i128         )
                ( FROM_ADDRESS_HI            :i32          )
                ( FROM_ADDRESS_LO            :i128         )
                ( IS_DEPLOYMENT              :binary@prove )
                ( NONCE                      :i64          ) ;; recall the EIP capping nonces to 2^64 - 1 or so
                ( VALUE                      :i128         )
                ( GAS_LIMIT                  :i32          )
                ( GAS_PRICE                  :i64          )
                ( GAS_INITIALLY_AVAILABLE    :i32          )
                ( CALL_DATA_SIZE             :i24          )
                ( INIT_CODE_SIZE             :i24          )
                ( HAS_EIP_1559_GAS_SEMANTICS :binary@prove )
                ( REQUIRES_EVM_EXECUTION     :binary@prove )
                ( COPY_TXCD                  :binary@prove )
                ( CFI                        :i16          )
                ( INIT_BALANCE               :i128         )
                ( STATUS_CODE                :binary@prove )
                ( GAS_LEFTOVER               :i32          )
                ( REFUND_COUNTER_FINAL       :i32          )
                ( REFUND_EFFECTIVE           :i32          )
                ( EIP_4788                   :binary@prove )
                ( EIP_2935                   :binary@prove )
                ( NOOP                       :binary@prove )
		( SYST_TXN_DATA              :i128 :display :bytes :array [5] )
		))
