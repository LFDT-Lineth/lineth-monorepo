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
                 ( btc_BLOCK_GAS_LIMIT        :i64          )
                 ( btc_BASEFEE                :i64          )
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
                 ( GAS_LIMIT                  :i25          )
                 ( GAS_PRICE                  :i64          )
                 ( GAS_INITIALLY_AVAILABLE    :i25          ) ;; EIP-7825 Transaction Gas Limit cap
                 ( CALL_DATA_SIZE             :i24          )
                 ( INIT_CODE_SIZE             :i24          )
                 ( TRANSACTION_TYPE_SUPPORTS_EIP_1559_GAS_SEMANTICS :binary@prove )
                 ( TRANSACTION_TYPE_SUPPORTS_DELEGATION_LISTS       :binary@prove )
                 ( REQUIRES_EVM_EXECUTION     :binary@prove )
                 ( COPY_TXCD                  :binary@prove )
                 ( CFI                        :i16          )
                 ( INIT_BALANCE               :i128         )
                 ( STATUS_CODE                :binary@prove )
                 ( GAS_LEFTOVER               :i25          )
                 ( REFUND_COUNTER_FINAL       :i25          )
                 ( REFUND_EFFECTIVE           :i25          )
                 ( EIP_4788                   :binary@prove )
                 ( EIP_2935                   :binary@prove )
                 ( NOOP                       :binary@prove )
                 ( SYST_TXN_DATA_1            :i64          )
                 ( SYST_TXN_DATA_2            :i16          )
                 ( SYST_TXN_DATA_3            :i128         )
                 ( SYST_TXN_DATA_4            :i128         )
                 ( SYST_TXN_DATA_5            :binary       )

                 ( LENGTH_OF_DELEGATION_LIST                :i10 )
                 ( NUMBER_OF_SUCCESSFUL_SENDER_DELEGATIONS  :i10 )

                 ))
