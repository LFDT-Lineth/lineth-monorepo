(defun (hub-into-rom-jump-destination-vetting-trigger)
  (and hub.PEEK_AT_STACK
       hub.stack/JUMP_DESTINATION_VETTING_REQUIRED))

(deflookup hub-into-rom-jump-destination-vetting
           ;; target columns
	   (
	     rom.CFI
	     rom.PC
	     rom.IS_JUMPDEST
           )
           ;; source columns
	   (
	     (* hub.CFI                                (hub-into-rom-jump-destination-vetting-trigger))
	     (* [hub.stack/STACK_ITEM_VALUE_LO 1]      (hub-into-rom-jump-destination-vetting-trigger))
	     (* (- 1 hub.stack/JUMPX)                  (hub-into-rom-jump-destination-vetting-trigger))
           )
)
