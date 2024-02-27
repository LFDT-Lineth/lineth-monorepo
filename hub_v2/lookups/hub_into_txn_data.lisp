(defun (hub-into-txn-data-trigger) hub_v2.PEEK_AT_TRANSACTION)

(deflookup hub-into-txn-data
           ;; target columns
           (
            txnData.ABS_TX_NUM
            txnData.BTC_NUM
            txnData.FROM_HI
            txnData.FROM_LO
            txnData.TO_HI
            txnData.TO_LO
            txnData.COINBASE_HI
            txnData.COINBASE_LO
            ;;
            txnData.NONCE
            txnData.VALUE
            txnData.IS_DEP
            txnData.TYPE2
            txnData.GAS_PRICE
            txnData.BASEFEE
            txnData.CALL_DATA_SIZE
            txnData.INIT_CODE_SIZE
            ;;
            txnData.INITIAL_GAS
            txnData.INITIAL_BALANCE
            txnData.REQUIRES_EVM_EXECUTION
            ;; txnData.COPY_TXCD_AT_INITIALIZATION  ;; TODO: uncomment
            txnData.STATUS_CODE
            txnData.LEFTOVER_GAS
            txnData.REFUND_COUNTER
            txnData.REFUND_AMOUNT
            )
           ;; source columns
           (
            (* hub_v2.ABSOLUTE_TRANSACTION_NUMBER                                      (hub-into-txn-data-trigger))
            (* hub_v2.transaction/BATCH_NUM                                            (hub-into-txn-data-trigger))
            (* hub_v2.transaction/FROM_ADDRESS_HI                                      (hub-into-txn-data-trigger))
            (* hub_v2.transaction/FROM_ADDRESS_LO                                      (hub-into-txn-data-trigger))
            (* hub_v2.transaction/TO_ADDRESS_HI                                        (hub-into-txn-data-trigger))
            (* hub_v2.transaction/TO_ADDRESS_LO                                        (hub-into-txn-data-trigger))
            (* hub_v2.transaction/COINBASE_ADDRESS_HI                                  (hub-into-txn-data-trigger))
            (* hub_v2.transaction/COINBASE_ADDRESS_LO                                  (hub-into-txn-data-trigger))
            ;;
            (* hub_v2.transaction/NONCE                                                (hub-into-txn-data-trigger))
            (* hub_v2.transaction/VALUE                                                (hub-into-txn-data-trigger))
            (* hub_v2.transaction/IS_DEPLOYMENT                                        (hub-into-txn-data-trigger))
            (* hub_v2.transaction/IS_TYPE2                                             (hub-into-txn-data-trigger))
            (* hub_v2.transaction/GAS_PRICE                                            (hub-into-txn-data-trigger))
            (* hub_v2.transaction/BASEFEE                                              (hub-into-txn-data-trigger))
            (* hub_v2.transaction/CALL_DATA_SIZE                                       (hub-into-txn-data-trigger))
            (* hub_v2.transaction/INIT_CODE_SIZE                                       (hub-into-txn-data-trigger))
            ;;
            (* hub_v2.transaction/INITIAL_GAS                                          (hub-into-txn-data-trigger))
            (* hub_v2.transaction/INITIAL_BALANCE                                      (hub-into-txn-data-trigger))
            (* hub_v2.transaction/REQUIRES_EVM_EXECUTION                               (hub-into-txn-data-trigger))
            ;; (* hub_v2.transaction/COPY_TXCD_AT_INITIALIZATION                          (hub-into-txn-data-trigger))    ;; TODO: uncomment
            (* hub_v2.transaction/STATUS_CODE                                          (hub-into-txn-data-trigger))
            (* hub_v2.transaction/LEFTOVER_GAS                                         (hub-into-txn-data-trigger))
            (* hub_v2.transaction/REFUND_COUNTER_INFINITY                              (hub-into-txn-data-trigger))
            (* hub_v2.transaction/REFUND_AMOUNT                                        (hub-into-txn-data-trigger))
            )
           )
