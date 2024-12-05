(deflookup
  blockhash-into-blockdata
  ; target columns
  (
    blockdata.REL_BLOCK
    blockdata.DATA_LO
    blockdata.INST
  )
  ; source columns
  (
    blockhash.REL_BLOCK
    blockhash.ABS_BLOCK
    (* blockhash.IOMF EVM_INST_NUMBER)
  ))


