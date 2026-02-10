(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    X.Y.Y RLP-view columns    ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defperspective rlp

	;; selector
	RLP

	;; RLP view columns
	(
	 ( TX_TYPE                            :i3           )
	 ( TYPE_0                             :binary@prove )
	 ( TYPE_1                             :binary@prove )
	 ( TYPE_2                             :binary@prove )
	 ( TYPE_3                             :binary@prove )
	 ( TYPE_4                             :binary@prove )
	 ( TO_ADDRESS_HI                      :i32          )
	 ( TO_ADDRESS_LO                      :i128         )
	 ( NONCE                              :i64          )
	 ( IS_DEPLOYMENT                      :binary@prove )
	 ( VALUE                              :i128         )
	 ( NUMBER_OF_ZERO_BYTES               :i32          )
	 ( NUMBER_OF_NONZERO_BYTES            :i32          )
	 ( DATA_SIZE                          :i24          )
	 ( INIT_SIZE                          :i24          )
	 ( GAS_LIMIT                          :i25          )
	 ( GAS_PRICE                          :i64          )
	 ( MAX_PRIORITY_FEE_PER_GAS           :i64          )
	 ( MAX_FEE_PER_GAS                    :i64          )
	 ( NUMBER_OF_ACCESS_LIST_ADDRESSES    :i24          )
	 ( NUMBER_OF_ACCESS_LIST_STORAGE_KEYS :i24          )
	 ( CHAIN_ID                           :i64          )
	 ( CFI                                :i16          )
	 ( REQUIRES_EVM_EXECUTION             :binary@prove )
	 ( NUMBER_OF_AUTHORIZATIONS           :i10          )
	 ))
