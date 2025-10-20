(defun (hub-into-bin-activation-flag)
  (* (unexceptional-stack-row)
      hub.stack/BIN_FLAG))

(defclookup hub-into-bin
  ;; target columns
  (
   bin.ARGUMENT_1
   bin.ARGUMENT_2
   bin.RES
   bin.INST
  )
  ;; source selector
  (hub-into-bin-activation-flag)
  ;; source columns
  (
   (:: [hub.stack/STACK_ITEM_VALUE_HI 1] [hub.stack/STACK_ITEM_VALUE_LO 1]) ;; arg1
   (:: [hub.stack/STACK_ITEM_VALUE_HI 2] [hub.stack/STACK_ITEM_VALUE_LO 2]) ;; arg2
   (:: [hub.stack/STACK_ITEM_VALUE_HI 4] [hub.stack/STACK_ITEM_VALUE_LO 4]) ;; result   
    hub.stack/INSTRUCTION
  )
)
