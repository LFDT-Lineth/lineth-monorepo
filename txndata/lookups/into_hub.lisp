(deflookup txn_data_into_hub
 ;; target columns
 (
  hub.BATCH_NUMBER
  hub.ABSOLUTE_TRANSACTION_NUMBER
 )
 ;; source columns
 (
  txndata.REL_BLOCK
  txndata.ABS_TX_NUM
 )
)
