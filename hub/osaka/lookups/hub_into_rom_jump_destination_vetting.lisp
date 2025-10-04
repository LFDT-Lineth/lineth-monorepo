(defun (hub-into-rom-jump-destination-vetting-trigger)
  (* hub.PEEK_AT_STACK
     hub.stack/JUMP_DESTINATION_VETTING_REQUIRED))

(defclookup
  (hub-into-rom-jump-destination-vetting :unchecked)
  ;; target columns
  (
   rom.CFI
   rom.PC
   rom.IS_JUMPDEST
  )
  ;; source selector
  (hub-into-rom-jump-destination-vetting-trigger)
  ;; source columns
  (
   hub.CFI
   [hub.stack/STACK_ITEM_VALUE_LO 1]
   (- 1 hub.stack/JUMPX)
  )
)
