(deflookup 
  rom-into-inst-decoder
  ;; target columns
  (
    instruction-decoder.OPCODE
    instruction-decoder.IS_PUSH
    instruction-decoder.IS_JUMPDEST
  )
  ;; source columns
  (
    rom.OPCODE
    rom.IS_PUSH
    rom.IS_JUMPDEST
  ))


