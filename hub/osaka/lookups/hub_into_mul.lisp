(defun (hub-into-mul-activation-flag)
  (* (unexceptional-stack-row)
      hub.stack/MUL_FLAG))

(defclookup hub-into-mul
  ;; target columns
  (
   mul.ARG_1
   mul.ARG_2
   mul.RES
   mul.INST
  )
  ;; source selector
  (hub-into-mul-activation-flag)
  ;; source columns
  (
   (:: [hub.stack/STACK_ITEM_VALUE_HI 1] [hub.stack/STACK_ITEM_VALUE_LO 1]) ;; arg1
   (:: [hub.stack/STACK_ITEM_VALUE_HI 2] [hub.stack/STACK_ITEM_VALUE_LO 2]) ;; arg2
   (:: [hub.stack/STACK_ITEM_VALUE_HI 4] [hub.stack/STACK_ITEM_VALUE_LO 4]) ;; result   
    hub.stack/INSTRUCTION
  )
)
