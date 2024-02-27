(defun (hub-into-rom-jump-destination-vetting-trigger)
  (and hub_v2.PEEK_AT_STACK
       hub_v2.stack/JUMP_DESTINATION_VETTING_REQUIRED))

(deflookup hub-into-gas
           ;; target columns
	   ( 
	     rom.CFI
	     rom.OPCODE
	     rom.IS_JUMPDEST
           )
           ;; source columns
	   (
	     (* hub_v2.CFI                    (hub-into-rom-jump-destination-vetting-trigger))
	     (* hub_v2.stack/INSTRUCTION      (hub-into-rom-jump-destination-vetting-trigger))
	     (* (- 1 hub_v2.stack/JUMPX)      (hub-into-rom-jump-destination-vetting-trigger))
           )
)
