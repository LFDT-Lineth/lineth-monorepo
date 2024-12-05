(defpurefun (hub-into-rom-instruction-fetching-trigger) hub.PEEK_AT_STACK)

(deflookup hub-into-rom-instruction-fetching
           ;; target columns
	   (
	     rom.CFI
	     rom.PC
	     rom.OPCODE
	     rom.PUSH_VALUE_HI
	     rom.PUSH_VALUE_LO
           )
           ;; source columns
	   (
	     (* hub.CFI                    (hub-into-rom-instruction-fetching-trigger))
	     (* hub.PC                     (hub-into-rom-instruction-fetching-trigger))
	     (* hub.stack/INSTRUCTION      (hub-into-rom-instruction-fetching-trigger))
	     (* hub.stack/PUSH_VALUE_HI    (hub-into-rom-instruction-fetching-trigger))
	     (* hub.stack/PUSH_VALUE_LO    (hub-into-rom-instruction-fetching-trigger))
           )
)
