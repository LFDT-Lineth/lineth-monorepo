(defun (hub-into-wcp-activation-flag)
  (* (unexceptional-stack-row)
      hub.stack/WCP_FLAG))

(defclookup hub-into-wcp
  ;; target columns
  (
   wcp.ARG_1_HI
   wcp.ARG_1_LO
   wcp.ARG_2_HI
   wcp.ARG_2_LO
   wcp.RESULT
   wcp.INST
  )
  ;; source selector
  (hub-into-wcp-activation-flag)
  ;; source columns
  (
   [hub.stack/STACK_ITEM_VALUE_HI 1]
   [hub.stack/STACK_ITEM_VALUE_LO 1]
   [hub.stack/STACK_ITEM_VALUE_HI 2]
   [hub.stack/STACK_ITEM_VALUE_LO 2]
   [hub.stack/STACK_ITEM_VALUE_LO 4]
   hub.stack/INSTRUCTION
  )
)
