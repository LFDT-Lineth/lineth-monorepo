(defpurefun (hub-into-rom-instruction-fetching-trigger) hub_v2.PEEK_AT_STACK)

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
	     (* hub_v2.CFI                    (hub-into-rom-instruction-fetching-trigger))
	     (* hub_v2.PC                     (hub-into-rom-instruction-fetching-trigger))
	     (* hub_v2.stack/INSTRUCTION      (hub-into-rom-instruction-fetching-trigger))
	     (* hub_v2.stack/PUSH_VALUE_HI    (hub-into-rom-instruction-fetching-trigger))
	     (* hub_v2.stack/PUSH_VALUE_LO    (hub-into-rom-instruction-fetching-trigger))
           )
)

