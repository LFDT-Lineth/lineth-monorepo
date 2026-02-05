(module hub)

(defun (hub-into-bin-activation-flag)
   (and! (unexceptional-stack-row-logical)
         (!= 0 hub.stack/BIN_FLAG)))

(defcall

  ;; result
  ((:: [hub.stack/STACK_ITEM_VALUE_HI 4] [hub.stack/STACK_ITEM_VALUE_LO 4])) ;; result

  bin

  ;; input
  (
   hub.stack/INSTRUCTION
   (:: [hub.stack/STACK_ITEM_VALUE_HI 1] [hub.stack/STACK_ITEM_VALUE_LO 1]) ;; arg1
   (:: [hub.stack/STACK_ITEM_VALUE_HI 2] [hub.stack/STACK_ITEM_VALUE_LO 2]) ;; arg2
  )
  ;; source selector
   (hub-into-bin-activation-flag)
)
