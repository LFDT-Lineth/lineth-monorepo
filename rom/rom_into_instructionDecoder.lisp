(defplookup 
  rom-into-inst-decoder
  ;reference columns
  (
    instruction-decoder.OPCODE
    instruction-decoder.IS_PUSH
    instruction-decoder.IS_JUMPDEST
  )
  ;source columns
  (
    rom.OPCODE
    rom.IS_PUSH
    rom.VALID_JUMP_DESTINATION
  ))


