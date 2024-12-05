(deflookup
  blockhash-into-wcp-lex
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
    (* (prev blockhash.BLOCK_NUMBER_HI) blockhash.IOMF)
    (* (prev blockhash.BLOCK_NUMBER_LO) blockhash.IOMF)
    blockhash.IOMF
    (* WCP_INST_GEQ blockhash.IOMF)
  ))


