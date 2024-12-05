(deflookup
  rom-into-instdecoder
  ;; target columns
  (
    instdecoder.OPCODE
    instdecoder.IS_PUSH
    instdecoder.IS_JUMPDEST
  )
  ;; source columns
  (
    rom.OPCODE
    rom.IS_PUSH
    rom.IS_JUMPDEST
  ))


