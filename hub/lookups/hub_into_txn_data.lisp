(defun (hub-into-txn-data-trigger) hub.PEEK_AT_TRANSACTION)

(deflookup hub-into-txndata
           ;; target columns
           (
            txndata.ABS_TX_NUM
            txndata.REL_BLOCK
            txndata.FROM_HI
            txndata.FROM_LO
            txndata.TO_HI
            txndata.TO_LO
            txndata.COINBASE_HI
            txndata.COINBASE_LO
            ;;
            txndata.NONCE
            txndata.VALUE
            txndata.IS_DEP
            txndata.TYPE2
            txndata.GAS_PRICE
            txndata.GAS_LIMIT
            txndata.PRIORITY_FEE_PER_GAS
            txndata.BASEFEE
            txndata.CALL_DATA_SIZE
            txndata.INIT_CODE_SIZE
            ;;
            txndata.GAS_INITIALLY_AVAILABLE
            txndata.INITIAL_BALANCE
            txndata.REQUIRES_EVM_EXECUTION
            txndata.COPY_TXCD
            txndata.STATUS_CODE
            txndata.GAS_LEFTOVER
            txndata.REFUND_COUNTER
            txndata.REFUND_EFFECTIVE
            )
           ;; source columns
           (
            (* hub.ABSOLUTE_TRANSACTION_NUMBER                                      (hub-into-txn-data-trigger))
            (* hub.RELATIVE_BLOCK_NUMBER                                            (hub-into-txn-data-trigger))
            (* hub.transaction/FROM_ADDRESS_HI                                      (hub-into-txn-data-trigger))
            (* hub.transaction/FROM_ADDRESS_LO                                      (hub-into-txn-data-trigger))
            (* hub.transaction/TO_ADDRESS_HI                                        (hub-into-txn-data-trigger))
            (* hub.transaction/TO_ADDRESS_LO                                        (hub-into-txn-data-trigger))
            (* hub.transaction/COINBASE_ADDRESS_HI                                  (hub-into-txn-data-trigger))
            (* hub.transaction/COINBASE_ADDRESS_LO                                  (hub-into-txn-data-trigger))
            ;;
            (* hub.transaction/NONCE                                                (hub-into-txn-data-trigger))
            (* hub.transaction/VALUE                                                (hub-into-txn-data-trigger))
            (* hub.transaction/IS_DEPLOYMENT                                        (hub-into-txn-data-trigger))
            (* hub.transaction/IS_TYPE2                                             (hub-into-txn-data-trigger))
            (* hub.transaction/GAS_PRICE                                            (hub-into-txn-data-trigger))
            (* hub.transaction/GAS_LIMIT                                            (hub-into-txn-data-trigger))
            (* hub.transaction/PRIORITY_FEE_PER_GAS                                 (hub-into-txn-data-trigger))
            (* hub.transaction/BASEFEE                                              (hub-into-txn-data-trigger))
            (* hub.transaction/CALL_DATA_SIZE                                       (hub-into-txn-data-trigger))
            (* hub.transaction/INIT_CODE_SIZE                                       (hub-into-txn-data-trigger))
            ;;
            (* hub.transaction/GAS_INITIALLY_AVAILABLE                              (hub-into-txn-data-trigger))
            (* hub.transaction/INITIAL_BALANCE                                      (hub-into-txn-data-trigger))
            (* hub.transaction/REQUIRES_EVM_EXECUTION                               (hub-into-txn-data-trigger))
            (* hub.transaction/COPY_TXCD                                            (hub-into-txn-data-trigger))
            (* hub.transaction/STATUS_CODE                                          (hub-into-txn-data-trigger))
            (* hub.transaction/GAS_LEFTOVER                                         (hub-into-txn-data-trigger))
            (* hub.transaction/REFUND_COUNTER_INFINITY                              (hub-into-txn-data-trigger))
            (* hub.transaction/REFUND_EFFECTIVE                                     (hub-into-txn-data-trigger))
            )
           )
