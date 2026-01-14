(defun (hub-into-rom-instruction-fetching-trigger) hub.PEEK_AT_STACK)

(defclookup hub-into-rom-instruction-fetching
  ;; target columns
  (
   rom.CFI
   rom.PC
   rom.OPCODE
   rom.PUSH_VALUE_HI
   rom.PUSH_VALUE_LO
  )
  ;; source selector
  (hub-into-rom-instruction-fetching-trigger)
  ;; source columns
  (
   hub.CFI
   hub.PC
   hub.stack/INSTRUCTION
   hub.stack/PUSH_VALUE_HI
   hub.stack/PUSH_VALUE_LO
  )
)
