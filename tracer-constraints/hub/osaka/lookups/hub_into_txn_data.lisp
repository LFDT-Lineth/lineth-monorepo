(defun ((txn-data---priority-fee-per-gas :force :i64))  (*  txndata.HUB   txndata.USER  (- txndata.hub/GAS_PRICE txndata.hub/btc_BASEFEE)))

;; ""

(defclookup (hub-into-txndata :unchecked)
  ;; target selector
  txndata.HUB
  ;; target columns
  (
   txndata.BLK_NUMBER
   txndata.TOTL_TXN_NUMBER
   ;; addresses
   (*   txndata.USER   txndata.hub/FROM_ADDRESS_HI            )
   (*   txndata.USER   txndata.hub/FROM_ADDRESS_LO            )
   (*   txndata.USER   txndata.hub/TO_ADDRESS_HI              )
   (*   txndata.USER   txndata.hub/TO_ADDRESS_LO              )
   (*   txndata.USER   txndata.hub/btc_COINBASE_ADDRESS_HI    )
   (*   txndata.USER   txndata.hub/btc_COINBASE_ADDRESS_LO    )
   ;; transaction parameters
   (*   txndata.USER   txndata.hub/NONCE                      )
   (*   txndata.USER   txndata.hub/VALUE                      )
   (*   txndata.USER   txndata.hub/IS_DEPLOYMENT              )
   (*   txndata.USER   txndata.hub/TRANSACTION_TYPE_SUPPORTS_EIP_1559_GAS_SEMANTICS )
   (*   txndata.USER   txndata.hub/TRANSACTION_TYPE_SUPPORTS_DELEGATION_LISTS       )
   (*   txndata.USER   txndata.hub/GAS_PRICE                  )
   (*   txndata.USER   txndata.hub/GAS_LIMIT                  )
   (txn-data---priority-fee-per-gas)
   (*   txndata.USER   txndata.hub/btc_BASEFEE                )
   (*   txndata.USER   txndata.hub/CALL_DATA_SIZE             )
   (*   txndata.USER   txndata.hub/INIT_CODE_SIZE             )
   ;; start of transaction data
   (*   txndata.USER   txndata.hub/GAS_INITIALLY_AVAILABLE    )
   (*   txndata.USER   txndata.hub/INIT_BALANCE               )
   (*   txndata.USER   txndata.hub/REQUIRES_EVM_EXECUTION     )
   (*   txndata.USER   txndata.hub/COPY_TXCD                  )
   ;; end of transaction data
   (*   txndata.USER   txndata.hub/STATUS_CODE                )
   (*   txndata.USER   txndata.hub/GAS_LEFTOVER               )
   (*   txndata.USER   txndata.hub/REFUND_COUNTER_FINAL       )
   (*   txndata.USER   txndata.hub/REFUND_EFFECTIVE           )
   ;;
   (*   txndata.USER   txndata.hub/LENGTH_OF_DELEGATION_LIST               )
   (*   txndata.USER   txndata.hub/NUMBER_OF_SUCCESSFUL_SENDER_DELEGATIONS )
   ;; system transaction stuff
   txndata.hub/NOOP
   txndata.hub/EIP_4788
   txndata.hub/EIP_2935
   txndata.hub/SYST_TXN_DATA_1
   txndata.hub/SYST_TXN_DATA_2
   txndata.hub/SYST_TXN_DATA_3
   txndata.hub/SYST_TXN_DATA_4
   txndata.hub/SYST_TXN_DATA_5

  )
  ;; source selector
  hub.PEEK_AT_TRANSACTION
  ;; source columns
  (
   hub.BLK_NUMBER
   hub.TOTL_TXN_NUMBER
   (*   hub.USER    hub.transaction/FROM_ADDRESS_HI         )
   (*   hub.USER    hub.transaction/FROM_ADDRESS_LO         )
   (*   hub.USER    hub.transaction/TO_ADDRESS_HI           )
   (*   hub.USER    hub.transaction/TO_ADDRESS_LO           )
   (*   hub.USER    hub.transaction/COINBASE_ADDRESS_HI     )
   (*   hub.USER    hub.transaction/COINBASE_ADDRESS_LO     )
   ;;
   (*   hub.USER    hub.transaction/NONCE                   )
   (*   hub.USER    hub.transaction/VALUE                   )
   (*   hub.USER    hub.transaction/IS_DEPLOYMENT           )
   (*   hub.USER    hub.transaction/TRANSACTION_TYPE_SUPPORTS_EIP_1559_GAS_SEMANTICS )
   (*   hub.USER    hub.transaction/TRANSACTION_TYPE_SUPPORTS_DELEGATION_LISTS       )
   (*   hub.USER    hub.transaction/GAS_PRICE               )
   (*   hub.USER    hub.transaction/GAS_LIMIT               )
   (*   hub.USER    hub.transaction/PRIORITY_FEE_PER_GAS    )
   (*   hub.USER    hub.transaction/BASEFEE                 )
   (*   hub.USER    hub.transaction/CALL_DATA_SIZE          )
   (*   hub.USER    hub.transaction/INIT_CODE_SIZE          )
   ;;
   (*   hub.USER    hub.transaction/GAS_INITIALLY_AVAILABLE )
   (*   hub.USER    hub.transaction/INITIAL_BALANCE         )
   (*   hub.USER    hub.transaction/REQUIRES_EVM_EXECUTION  )
   (*   hub.USER    hub.transaction/COPY_TXCD               )
   ;;
   (*   hub.USER    hub.transaction/STATUS_CODE             )
   (*   hub.USER    hub.transaction/GAS_LEFTOVER            )
   (*   hub.USER    hub.transaction/REFUND_COUNTER_INFINITY )
   (*   hub.USER    hub.transaction/REFUND_EFFECTIVE        )
   ;;
   (*   hub.USER    hub.transaction/LENGTH_OF_DELEGATION_LIST               )
   (*   hub.USER    hub.transaction/NUMBER_OF_SUCCESSFUL_SENDER_DELEGATIONS )
   ;;
   hub.transaction/NOOP
   hub.transaction/EIP_4788
   hub.transaction/EIP_2935
   hub.transaction/SYST_TXN_DATA_1
   hub.transaction/SYST_TXN_DATA_2
   hub.transaction/SYST_TXN_DATA_3
   hub.transaction/SYST_TXN_DATA_4
   hub.transaction/SYST_TXN_DATA_5
  )
)
