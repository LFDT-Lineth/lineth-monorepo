(deflookup txndata-into-hub
 ;; target columns
 (
  hub.RELATIVE_BLOCK_NUMBER
  hub.ABSOLUTE_TRANSACTION_NUMBER
 )
 ;; source columns
 (
  txndata.REL_BLOCK
  txndata.ABS_TX_NUM
 )
)
