(defun (hub-into-shf-activation-flag)
  (* (unexceptional-stack-row)
      hub.stack/SHF_FLAG))

(defclookup hub-into-shf
  ;; target columns
  (
   shf.ARG_1
   shf.ARG_2
   shf.RES
   shf.INST
  )
  ;; source selector
  (hub-into-shf-activation-flag)
  ;; source columns
  (
   (:: [hub.stack/STACK_ITEM_VALUE_HI 1] [hub.stack/STACK_ITEM_VALUE_LO 1]) ;; arg1
   (:: [hub.stack/STACK_ITEM_VALUE_HI 2] [hub.stack/STACK_ITEM_VALUE_LO 2]) ;; arg2
   (:: [hub.stack/STACK_ITEM_VALUE_HI 4] [hub.stack/STACK_ITEM_VALUE_LO 4]) ;; result   
   hub.stack/INSTRUCTION
  )
)
