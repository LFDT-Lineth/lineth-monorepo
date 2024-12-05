(deflookup
  blockhash-into-wcp-lower-bound
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
    (* (- blockhash.ABS_BLOCK 256) blockhash.IOMF)
    (* blockhash.LOWER_BOUND_CHECK blockhash.IOMF)
    (* WCP_INST_GEQ blockhash.IOMF)
  ))


