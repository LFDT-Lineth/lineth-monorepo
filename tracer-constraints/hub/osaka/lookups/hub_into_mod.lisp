(defun (hub-into-mod-activation-flag)
  (* (unexceptional-stack-row)
      hub.stack/MOD_FLAG))

(defclookup hub-into-mod
  ;; target columns
  (
   mod.ARG_1
   mod.ARG_2
   mod.RES
   mod.INST
  )
  ;; source selector
  (hub-into-mod-activation-flag)
  ;; source columns
  (
   (:: [hub.stack/STACK_ITEM_VALUE_HI 1] [hub.stack/STACK_ITEM_VALUE_LO 1]) ;; arg1
   (:: [hub.stack/STACK_ITEM_VALUE_HI 2] [hub.stack/STACK_ITEM_VALUE_LO 2]) ;; arg2
   (:: [hub.stack/STACK_ITEM_VALUE_HI 4] [hub.stack/STACK_ITEM_VALUE_LO 4]) ;; result
    hub.stack/INSTRUCTION
  )
)
