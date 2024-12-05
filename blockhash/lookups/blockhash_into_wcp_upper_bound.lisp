(deflookup
  blockhash-into-wcp-upper-bound
  ; target columns
  (
    wcp.ARGUMENT_1_HI
    wcp.ARGUMENT_1_LO
    wcp.ARGUMENT_2_HI
    wcp.ARGUMENT_2_LO
    wcp.RESULT
    wcp.INST
  )
  ; source columns
  (
    (* blockhash.BLOCK_NUMBER_HI blockhash.IOMF)
    (* blockhash.BLOCK_NUMBER_LO blockhash.IOMF)
    0
    (* blockhash.ABS_BLOCK blockhash.IOMF)
    (* blockhash.UPPER_BOUND_CHECK blockhash.IOMF)
    (* EVM_INST_LT blockhash.IOMF)
  ))


