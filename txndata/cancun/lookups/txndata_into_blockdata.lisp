(deflookup
  txndata-into-blockdata
  ; target columns
  (
    blockdata.REL_BLOCK
    blockdata.COINBASE_HI
    blockdata.COINBASE_LO
    blockdata.BASEFEE
    blockdata.BLOCK_GAS_LIMIT
  )
  ; source columns
  (
    txndata.REL_BLOCK
    txndata.COINBASE_HI
    txndata.COINBASE_LO
    txndata.BASEFEE
    txndata.BLOCK_GAS_LIMIT
  ))


