(defun (hub-into-ext-activation-flag)
  (* (unexceptional-stack-row)
      hub.stack/EXT_FLAG))

(defclookup hub-into-ext
  ;; target columns
  (
   ext.A
   ext.B
   ext.M
   ext.RES
   ext.INST
  )
  ;; source selector
  (hub-into-ext-activation-flag)
  ;; source columns
  (
   (:: [hub.stack/STACK_ITEM_VALUE_HI 1] [hub.stack/STACK_ITEM_VALUE_LO 1])
   (:: [hub.stack/STACK_ITEM_VALUE_HI 2] [hub.stack/STACK_ITEM_VALUE_LO 2])
   (:: [hub.stack/STACK_ITEM_VALUE_HI 3] [hub.stack/STACK_ITEM_VALUE_LO 3])
   (:: [hub.stack/STACK_ITEM_VALUE_HI 4] [hub.stack/STACK_ITEM_VALUE_LO 4])
    hub.stack/INSTRUCTION
  )
)
